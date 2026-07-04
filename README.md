# Sagittarius

**Sagittarius** is Layer 1 of the **Prediction Market Intelligence Engine (PMIE)** — a stateless Go [Model Context Protocol (MCP)](https://modelcontextprotocol.io) server that exposes Polymarket data as typed, LLM-consumable tools.

It is the data-access tier of a larger system whose goal is to explain *why* prediction markets move, not just show that they moved. Rather than dumping raw trades and order books into an LLM's context window, Sagittarius fetches and condenses Polymarket data into structured, semantically dense payloads that a reasoning agent can consume cheaply and reliably.

## Why this exists

Prediction markets like Polymarket are real-time sentiment aggregators, but dashboards only show the *what* (price charts) — never the *why*. Feeding raw event streams straight into an LLM is expensive and unreliable: context dilution, high token cost, and hallucinated explanations.

The PMIE's answer is a layered pipeline that keeps the LLM out of the data-fetching and math paths entirely:

```
Price Moved
   │
   ▼
[Layer 1: Polymarket MCP Server] ──(Raw Data)──> [Layer 2: Signal Engine (Deterministic)]
                                                           │
                                                   (Scored Signals)
                                                           │
                                                           ▼
[Layer 4: Reasoning Agent] <──(Semantic Context)── [Layer 3: Memory Layer (Vector/KV)]
   │
   ├──> Synthesizes Causal Explanations (Structured JSON Output)
   │
   ▼
[Layer 5: Evaluation Engine] ──> Continuously Tracks & Updates Historical Confidence Scores
```

| Layer | Responsibility | Home | Status |
|---|---|---|---|
| **1. MCP Server** | Stateless, protocol-compliant tool abstraction over Polymarket's APIs. Zero LLM awareness. | **This repo** | **Active** |
| **2. Signal Engine** | Deterministic math over raw data — whale detection, volume spikes, order book skew. | **This repo** (in-process with Layer 1; scored signals exposed as MCP tools) | Prototyped, being wired in |
| 3. Memory Layer | Vector/KV store of past signals and explanations; token-optimizes recurring context. | Cygnus repo | Planned |
| 4. Reasoning Agent | Google ADK agent that synthesizes causal explanations from signals + memory. | Cygnus repo | Active |
| 5. Evaluation Engine | Scheduled worker that back-tests explanations against real outcomes and re-weights confidence. | Cygnus repo | Planned |

The 5 layers deliberately map onto **two repos** — Sagittarius (Go, Layers 1+2) and [Cygnus](https://github.com/TheGh0xt/Cygnus) (Python/ADK, Layers 3+4+5) — split by runtime and synchronous coupling. The MCP boundary between them is the only cross-repo seam.

See [`docs/docs_PROJECT_PROPOSAL.md`](./docs/docs_PROJECT_PROPOSAL.md) for the full vision and roadmap, and [`docs/docs_MCP_SERVER_SPEC.md`](./docs/docs_MCP_SERVER_SPEC.md) for the target protocol specification this layer is converging toward.

![Architecture Diagram](./assets/2026-06-30_pmie-full-architecture-diagram.png)

## What Sagittarius does today

Sagittarius runs as an MCP server over **stdio**, **Streamable HTTP**, or **SSE**, and currently exposes one production tool:

- **`get_event_by_slug`** — given a Polymarket event slug (the path segment after `/event/` in a Polymarket URL, e.g. `will-btc-hit-150k`), fetches the event from the Gamma API and returns a condensed `EventIntelligenceContext`: summary metrics (volume, liquidity, open interest), per-market analysis (implied probability, 1h/24h/7d/30d price change, best bid/ask, spread), tags, and event context.

This keeps a full, deeply-nested Gamma API payload from ever reaching the LLM — the tool handler returns only the fields relevant to reasoning about market movement.

## Architecture

Sagittarius follows a clean/hexagonal architecture with a strict one-way dependency flow:

```
MCP Client (Claude Desktop, an agent host, etc.)
  │  MCP protocol (stdio / SSE / streamable HTTP)
  ▼
cmd/sagittarius-mcp/main.go        — flag parsing, transport selection, wiring
  ▼
internal/interface/mcp/            — Server: transport adapters + tool registration
  ▼
internal/interface/mcp/tools/      — Pmhandler: MCP tool handlers
  ▼
internal/application/polymarket/   — Service: orchestration, builds EventIntelligenceContext
  ▼
internal/domain/polymarket/        — EventProvider interface, Event domain type
  ▼
internal/infrastructure/polymarket/— HTTP client → Polymarket Gamma/Data/CLOB APIs
```

### Package responsibilities

| Package | Role |
|---|---|
| `internal/interface/mcp/` | Transport wiring (stdio / SSE / streamable HTTP) and tool registration via `RegisterPmiTools()` |
| `internal/interface/mcp/tools/` | `Pmhandler` — converts raw service results into `mcp.CallToolResult` |
| `internal/application/polymarket/` | `Service` interface + `pmService` orchestration; request/response DTOs; `BuildEventIntelligenceContext` formatter |
| `internal/domain/polymarket/` | `EventProvider` repository interface; `Event` domain struct mirroring the Gamma API schema |
| `internal/domain/shared/` | Typed error structs (`ErrInvalidInput`, `ErrInternalServerError`) and JSON marshal/unmarshal helpers |
| `internal/infrastructure/polymarket/` | `Client` implementing `EventProvider`; generic `makePmGetRequest[T]` / `makePmPostRequest[T]` HTTP helpers |

Each layer only knows about the one below it via interfaces (e.g. the application layer depends on `domain.EventProvider`, never on the concrete infrastructure `Client`), so the HTTP client, transport, and reasoning layers can all evolve independently.

### Prototype packages (not wired to `main`)

`internal/polymarket/`, `internal/mcp/`, and `internal/signal/` are an earlier Phase 1 prototype kept in-tree for reference:

- `internal/mcp/server.go` — entirely commented out.
- `internal/signal/engine.go` — a deterministic Layer 2 Signal Engine (whale detection, order book skew, volume spike analysis) that still imports the old `internal/polymarket/` client. It is functional and unit-tested (`internal/signal/engine_test.go`) but not yet integrated into the active server built from `internal/interface/...`.

When extending Sagittarius, new work should follow the clean-architecture path (`domain` → `infrastructure` → `application` → `interface/mcp/tools` → register in `internal/interface/mcp/register.go`), not the prototype packages.

## Polymarket APIs

| API | Base URL | Used for |
|---|---|---|
| Gamma | `https://gamma-api.polymarket.com` | Events, markets, metadata (primary discovery) |
| Data | `https://data-api.polymarket.com` | Trades, positions, holders |
| CLOB | `https://clob-api.polymarket.com` | Orderbook snapshots, price history, midpoints |

All public read-only endpoints used here are unauthenticated — no API key is required to run this server.

## Getting started

### Prerequisites

- Go 1.26+
- [`just`](https://github.com/casey/just) (optional, for the recipes below)

### Build

```bash
just build
# or directly:
go build -o bin/sagittarius-mcp cmd/sagittarius-mcp/main.go
```

### Run

Sagittarius supports three transports, selected with `--transport`:

```bash
just run-http                                # Streamable HTTP on :8080 → POST /mcp
just run-sse                                 # SSE on :8080 → /sse + /message
./bin/sagittarius-mcp --transport=stdio      # stdio, for Claude Desktop / MCP clients
```

Both HTTP and SSE modes expose a health check:

```bash
curl http://localhost:8080/health
```

### Test

```bash
go test ./...
go test ./internal/signal/...   # signal engine unit tests only
```

### Calling the tool

Once connected via an MCP client, call `get_event_by_slug` with the slug from a Polymarket event URL:

```json
{
  "name": "get_event_by_slug",
  "arguments": { "slug": "will-btc-hit-150k" }
}
```

which returns an `EventIntelligenceContext`:

```json
{
  "event": {
    "title": "Will BTC hit $150k?",
    "volume": 1234567.0,
    "volume_24h": 45678.0,
    "liquidity": 89000.0,
    "open_interest": 250000.0
  },
  "context": "...",
  "tags": ["Crypto", "Bitcoin"],
  "markets": [
    {
      "question": "Will BTC hit $150k by Dec 2026?",
      "probability": 0.58,
      "change_1h": 0.01,
      "change_24h": -0.03,
      "change_7d": 0.12,
      "change_30d": 0.20,
      "volume_24h": 45678.0,
      "liquidity": 89000.0,
      "best_bid": 0.57,
      "best_ask": 0.59,
      "spread": 0.02
    }
  ]
}
```

## Contributing / next steps

The full planned tool surface (from [`docs/docs_MCP_SERVER_SPEC.md`](./docs/docs_MCP_SERVER_SPEC.md)) includes `get_event_by_id`, `search_markets`, `get_market_prices`, `get_market_history`, `get_market_orderbook`, `get_market_trades`, `get_whale_activity`, and `get_market_snapshot`. See [`docs/implementation_plan.md`](./docs/implementation_plan.md) for the original Phase 1 build-out plan.

### Repo conventions

- Direct pushes to `main` are blocked by the pre-push hook in `scripts/hooks/pre-push`; install it locally with:
  ```bash
  cp scripts/hooks/pre-push .git/hooks/pre-push && chmod +x .git/hooks/pre-push
  ```
- Add new tools by following the clean architecture path: domain types in `internal/domain/`, infrastructure calls in `internal/infrastructure/`, service logic in `internal/application/`, handlers in `internal/interface/mcp/tools/`, then register in `internal/interface/mcp/register.go`.
