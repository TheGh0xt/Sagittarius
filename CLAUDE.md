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

Sagittarius is Layer 1 of a 5-layer Prediction Market Intelligence Engine (PMIE). It is a stateless Go MCP server that exposes Polymarket data as typed tools to an LLM reasoning agent upstream.

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

### Prototype packages (not wired to main)

`internal/polymarket/`, `internal/mcp/`, and `internal/signal/` are the original Phase 1 prototype. `internal/mcp/server.go` is entirely commented out. `internal/signal/engine.go` (whale detection, orderbook skew, volume spikes) still imports `internal/polymarket/` and is the planned Layer 2 Signal Engine — not yet integrated into the active server.

When adding new tools, follow the clean architecture path: add domain types in `internal/domain/`, infrastructure calls in `internal/infrastructure/`, service logic in `internal/application/`, handlers in `internal/interface/mcp/tools/`, and register in `internal/interface/mcp/register.go`.

## Polymarket APIs

| API | Base URL | Used for |
|---|---|---|
| Gamma | `https://gamma-api.polymarket.com` | Events, markets, metadata (primary discovery) |
| Data | `https://data-api.polymarket.com` | Trades, positions, holders |
| CLOB | `https://clob-api.polymarket.com` | Orderbook snapshots, price history, midpoints |

All public read-only endpoints are unauthenticated. Event slugs are the path segment after `/event/` in Polymarket URLs (e.g. `will-btc-hit-150k`).

## Currently registered MCP tools

Only one tool is registered in production today (`internal/interface/mcp/register.go`):

- `get_event_by_slug` — fetches a Gamma event and returns a condensed `EventIntelligenceContext` (summary, markets with probabilities and price changes, tags, metadata context)

The full planned tool list (from `docs/docs_MCP_SERVER_SPEC.md`) includes `get_event_by_id`, `search_markets`, `get_market_prices`, `get_market_history`, `get_market_orderbook`, `get_market_trades`, `get_whale_activity`, and `get_market_snapshot`.

## Commit Conventions
- Never add "Co-Authored-By" lines or AI attribution to Git commits.
- Do not include Claude metadata in PR descriptions.
