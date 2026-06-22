# DOCUMENT 2: MODEL CONTEXT PROTOCOL (MCP) SERVER TECHNICAL SPECIFICATION
**File Path:** `docs/MCP_SERVER_SPEC.md`  
**Audience:** Core Backend Engineers, AI Coding Assistants (Cursor, Claude Code, Zed), Infrastructure Contributors  
**Status:** Approved / Proposed Specification Reference  

---

## 1. System Topology & Protocol Compliance
The Polymarket MCP Server is designed as an independent, stateless microservice communicating over **Standard Input/Output (stdio)** or **Server-Sent Events (SSE)** transport layers matching the Anthropic Model Context Protocol specification. 

```
┌────────────────────────┐                    ┌───────────────────────────┐
│                        │    MCP Protocol    │                           │
│     Reasoning Agent    │ <────────────────> │   Polymarket MCP Server   │
│ (ADK / Host Executive) │   (Stdio / SSE)    │   (Go / Rust Binary)      │
│                        │                    └─────────────┬─────────────┘
└────────────────────────┘                                  │
                                                       HTTPS│(Polymarket API/
                                                            │ RPC Nodes)
                                                            ▼
                                                   ┌───────────────────────────┐
                                                   │    Polymarket CLOB /      │
                                                   │    On-Chain Data Feeds    │
                                                   └───────────────────────────┘
```
### Core Design Requirements:
* **Language:** Go (using `github.com/modelcontextprotocol/go-sdk/mcp`).
* **Concurrency:** Thread-safe connection pools; must support parallel non-blocking tool execution using async/await patterns.
* **Error Handling:** Standardized JSON-RPC 2.0 error codes (`-32602` for Invalid Params, `-32603` for Internal Server Error).

---

## 2. API Tool Declarations (Strict JSON Schemas)
The MCP Server exposes the following programmatic tools to the Host LLM. Each tool must return a standard payload wrapping a text or structural JSON content array.

### 2.1 Market Discovery & Details
* `search_markets`: Query active or resolved prediction markets using text strings or category filters.
* `get_market`: Retrieve extensive metadata for a specific unique market ID (slug, contract address, end dates, options tokens).

### 2.2 Financial & Order Book Metrics
* `get_market_prices`: Fetch real-time token implied probabilities (e.g., Yes token = $0.58).
* `get_market_history`: Fetch time-series historical price bars (OHLCV) for market charting.
* `get_market_volume`: Fetch consolidated rolling transactional volume metrics over specified temporal intervals (1h, 24h, 7d).
* `get_market_orderbook`: Retrieve current Central Limit Order Book (CLOB) snapshots up to a depth of 50 levels (bids, asks, sizes).

### 2.3 Transactional & Whale Monitoring
* `get_market_trades`: Stream or paginate real-time execution logs for a distinct market ID.
* `get_market_holders`: Extract top token distribution matrices for position sizing validation.
* `get_whale_activity`: Retrieve structured lists of transactions exceeding defined financial boundaries (e.g., swaps > $50,000 USD equivalent).
* `get_market_snapshot`: Extract a unified state vector bundling price, depth skew, and recent velocity flags into a single payload.

#### Example: Tool Schema Definition (`get_whale_activity`)
```json
{
  "name": "get_whale_activity",
  "description": "Extract single trades or high-volume position modifications exceeding a configured fiat threshold for a specified market.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "market_id": {
        "type": "string",
        "description": "The unique identifying cryptographic address or slug for the target market."
      },
      "usd_threshold": {
        "type": "number",
        "description": "Minimum dollar value equivalent to classify transaction as a whale event. Defaults to 25000.",
        "default": 25000
      },
      "limit": {
        "type": "integer",
        "description": "Max count of events to fetch.",
        "default": 10
      }
    },
    "required": ["market_id"]
  }
}
```

---

## 3. Resource URIs & Semantic Context Map
Resources allow the host agent to read unstructured or structured data states continuously. The server exposes reactive URIs:

| Resource URI Template | Content Type | Operational Description |
| :--- | :--- | :--- |
| `market://active` | `application/json` | Real-time stream of all open, high-liquidity markets. |
| `market://trending` | `application/json` | Tracks markets with the fastest rate of change in volume/traders. |
| `market://resolved` | `application/json` | Historic log of recently finalized markets for evaluation cycles. |
| `market://market/{id}` | `application/json` | Static and structural state metadata for a distinct contract ID. |
| `market://signals/{id}` | `application/json` | Live feed of parsed metrics coming straight out of the Signal Engine. |

---

## 4. Structured Prompts (Context Injections)
The MCP Server supplies structural agent templates to enforce uniform reasoning and configuration parameters.

### `analyze_market`
* **Target Audience:** Prompt Router
* **System Prompt Core Injection:**
  ```text
  You are an expert quantitative crypto-analyst. Examine the target market metadata, current order book spread, and historical token velocity provided below. Isolate structural market anomalies and outline a data-gathering checklist for downstream processes.
  ```

### `explain_price_movement`
* **Target Audience:** Core Reasoning Agent
* **System Prompt Core Injection:**
  ```text
  A sharp variance in market probability has occurred. Cross-reference the provided token transaction logs, holder concentration metrics, and whale alerts to build a verifiable, data-backed causal hypothesis. Avoid descriptive prose; focus strictly on structured causative links.
  ```

### `detect_anomalies` / `summarize_market`
* Enforces output sanitization rules preventing the agent from hallucinating unverified trading activity.
