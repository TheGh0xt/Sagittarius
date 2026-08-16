# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
just build
# or: go build -o bin/sagittarius-mcp cmd/sagittarius-mcp/main.go

# Run (choose transport)
just run-http          # Streamable HTTP on :8080 → POST /mcp
just run-sse           # SSE on :8080 → /sse + /message
./bin/sagittarius-mcp --transport=stdio   # stdio mode for Claude Desktop / MCP clients

# Test
go test ./...
go test ./internal/signal/...  # signal engine unit tests only

# Health check (HTTP/SSE modes)
curl http://localhost:8080/health
```

The pre-push hook in `scripts/hooks/pre-push` blocks direct pushes to `main`. Install it if it's missing: `cp scripts/hooks/pre-push .git/hooks/pre-push && chmod +x .git/hooks/pre-push`.

## Architecture

Sagittarius is the permanent home of **Layers 1 and 2** of the 5-layer Prediction Market Intelligence Engine (PMIE). It is a stateless Go MCP server that exposes Polymarket data as typed tools to an LLM reasoning agent upstream (Cygnus, a separate Python/ADK repo housing Layers 3–5).

Layers 1 and 2 share this repo deliberately: both are deterministic, LLM-free, and on the hot path, so signal detection must be an in-process Go library call — never a network/MCP hop. The Signal Engine must also sit *between* the raw Polymarket fetch and the MCP surface, because raw trade/order-book data must never be exposed to the LLM; its scored signals get exposed as additional MCP tools (e.g. `get_whale_activity`) for Cygnus. The MCP boundary is the only seam between the two repos — Sagittarius never writes to Cygnus's memory store directly. Do not split Layer 2 into its own repo or service.

### Request flow (active code path)

```
MCP Client
  → cmd/sagittarius-mcp/main.go   (flag parsing, transport selection)
  → internal/interface/mcp/       (Server: transport adapters + tool registration)
  → internal/interface/mcp/tools/ (Pmhandler: tool handlers)
  → internal/application/polymarket/ (pmService: orchestration, builds EventIntelligenceContext)
  → internal/domain/polymarket/   (EventProvider interface, Event domain type)
  → internal/infrastructure/polymarket/ (HTTP client → Polymarket Gamma/Data/CLOB APIs)
```

### Layer responsibilities

| Package | Role |
|---|---|
| `internal/interface/mcp/` | Transport wiring (stdio / SSE / streamable HTTP), tool registration via `RegisterPmiTools()` |
| `internal/interface/mcp/tools/` | `Pmhandler` — converts raw service results to `mcp.CallToolResult` |
| `internal/application/polymarket/` | `Service` interface + `pmService`; DTOs (`FetchEventBySlugRequest`, `EventIntelligenceContext`); `BuildEventIntelligenceContext` formatter |
| `internal/domain/polymarket/` | `EventProvider` interface (repository pattern); `Event` domain struct (flat Gamma API schema) |
| `internal/domain/shared/` | Typed error structs (`ErrInvalidInput`, `ErrInternalServerError`), `MarshalJSON`/`UnmarshalJSON` |
| `internal/infrastructure/polymarket/` | `Client` implementing `EventProvider`; generic `makePmGetRequest[T]` / `makePmPostRequest[T]` helpers |

### Signal Engine (Layer 2) — wired in

The Signal Engine lives on the clean-architecture stack: pure deterministic math (whale detection, orderbook skew, volume-spike analysis) in `internal/domain/signal/` (no I/O — unit-tested without mocks), orchestration in `internal/application/signal/` (resolves an event slug to per-market condition/token IDs, fetches trades and order books, condenses them into `WhaleActivityReport` / `MarketSnapshotReport`), handlers in `internal/interface/mcp/tools/signal.go`. The original Phase 1 prototype packages (`internal/polymarket/`, `internal/signal/`, `internal/mcp/`) were deleted once this port landed.

When adding new tools, follow the clean architecture path: add domain types in `internal/domain/`, infrastructure calls in `internal/infrastructure/`, service logic in `internal/application/`, handlers in `internal/interface/mcp/tools/`, and register in `internal/interface/mcp/register.go`.

## Polymarket APIs

| API | Base URL | Used for |
|---|---|---|
| Gamma | `https://gamma-api.polymarket.com` | Events, markets, metadata (primary discovery) |
| Data | `https://data-api.polymarket.com` | Trades, positions, holders |
| CLOB | `https://clob.polymarket.com` | Orderbook snapshots, price history, midpoints (public `/book`; `/trades` requires L2 auth — use the Data API for trades) |

All public read-only endpoints are unauthenticated. Event slugs are the path segment after `/event/` in Polymarket URLs (e.g. `will-btc-hit-150k`).

## Currently registered MCP tools

Registered in `internal/interface/mcp/register.go`:

- `get_event_by_slug` `{slug}` — condensed `EventIntelligenceContext` (summary, markets with probabilities and price changes, tags, metadata context)
- `get_event_by_id` `{id}` — same payload, fetched by numeric Gamma event ID
- `search_markets` `{query, limit?=5}` — resolves free text to candidate event slugs ranked by volume, so a request naming a subject rather than a market can reach one; the caller then fetches the winner with `get_event_by_slug`. Only open events are returned — a settled market's price can never move again. Queries are reduced to their distinctive terms first (`internal/application/polymarket/query.go`), because Gamma matches phrases and a raw question scores against every other question-shaped market.
- `get_whale_activity` `{slug, usd_threshold?=25000, limit?=100}` — whale-sized trades per market with totals and buy/sell ratio
- `get_market_snapshot` `{slug}` — per-market state vector: implied probability, orderbook skew, volume-spike analysis, whale count

Still planned (from `docs/docs_MCP_SERVER_SPEC.md`): `get_market_prices`, `get_market_history`, `get_market_orderbook`, `get_market_trades`.

## Commit Conventions
- Never add "Co-Authored-By" lines or AI attribution to Git commits.
- Do not include Claude metadata in PR descriptions.
