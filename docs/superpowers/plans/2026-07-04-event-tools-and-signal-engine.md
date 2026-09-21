# Sagittarius: Event Tools + Signal Engine Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Register `get_event_by_id`, and port the prototype Signal Engine (Layer 2) onto the clean architecture, exposing `get_whale_activity` and `get_market_snapshot` as MCP tools.

**Architecture:** Follow the existing clean-architecture path: domain types/interfaces in `internal/domain/`, HTTP calls in `internal/infrastructure/polymarket/`, orchestration in `internal/application/`, MCP handlers in `internal/interface/mcp/tools/`, registration in `internal/interface/mcp/register.go`. The Signal Engine's math lives as pure functions in `internal/domain/signal/` (no I/O — unit-testable without mocks). Prototype packages `internal/polymarket/`, `internal/signal/`, `internal/mcp/` are deleted at the end once superseded.

**Tech Stack:** Go 1.26, `github.com/modelcontextprotocol/go-sdk/mcp`, stdlib `net/http/httptest` for infra tests.

## Global Constraints

- Work on the existing branch `feat/polymarket-service`. Never push to `main` (pre-push hook enforces).
- **Never add "Co-Authored-By" lines or any AI attribution to git commits.**
- Raw trade/order-book data must never be returned un-condensed: tool outputs are aggregated/filtered payloads (whale-filtered trades, computed skew), never full raw dumps.
- Run `go build ./...` and `go test ./...` before every commit; both must pass.
- Error conventions: validation failures return `shared.ErrInvalidInput{Field, Message}`; wrap upstream failures as-is (service layer passes through).
- Logging: every constructor takes `*slog.Logger` like existing code.

## Cross-Repo MCP Tool Contract (MUST match exactly — Cygnus depends on these)

| Tool | Input schema (JSON) | Output |
|---|---|---|
| `get_event_by_id` | `{"id": string}` — numeric Gamma event ID | `EventIntelligenceContext` (same shape as `get_event_by_slug`) |
| `get_whale_activity` | `{"slug": string, "usd_threshold": number (default 25000), "limit": integer (default 100)}` | `WhaleActivityReport` (below) |
| `get_market_snapshot` | `{"slug": string}` | `MarketSnapshotReport` (below) |

---

### Task 1: `get_event_by_id` — domain + infrastructure

**Files:**
- Modify: `internal/domain/polymarket/repository.go`
- Modify: `internal/infrastructure/polymarket/client.go` (URL builder)
- Modify: `internal/infrastructure/polymarket/gamma.go`
- Test: `internal/infrastructure/polymarket/gamma_test.go` (create)

**Interfaces:**
- Produces: `EventProvider.FetchEventByID(ctx context.Context, id string) (*polymarket.Event, error)`; Gamma URL `GET {base}/events/{id}`.
- Note: to make the base URL injectable for httptest, add a `baseGammaURL string` field to `Client` defaulting to `GammaBaseURL`, plus `NewClientWithBaseURLs(slg *slog.Logger, gammaURL string) *Client` used only by tests.

- [ ] **Step 1: Write the failing test**

```go
package polymarket

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchEventByID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/events/903193" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"903193","title":"Fed decision in October","volume":123.5}`))
	}))
	defer srv.Close()

	c := NewClientWithBaseURLs(slog.Default(), srv.URL)
	ev, err := c.FetchEventByID(context.Background(), "903193")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.ID != "903193" || ev.Title != "Fed decision in October" || ev.Volume != 123.5 {
		t.Errorf("unexpected event: %+v", ev)
	}
}

func TestFetchEventByIDUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found","message":"no event"}`))
	}))
	defer srv.Close()

	c := NewClientWithBaseURLs(slog.Default(), srv.URL)
	if _, err := c.FetchEventByID(context.Background(), "0"); err == nil {
		t.Fatal("expected error, got nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails** — `go test ./internal/infrastructure/polymarket/ -run TestFetchEventByID -v` → FAIL (`NewClientWithBaseURLs` / `FetchEventByID` undefined).

- [ ] **Step 3: Implement.** In `client.go`: add `baseGammaURL string` to `Client`; `NewClient` sets it to `GammaBaseURL`; add:

```go
// NewClientWithBaseURLs returns a *Client pointed at a custom Gamma base URL (tests).
func NewClientWithBaseURLs(slg *slog.Logger, gammaURL string) *Client {
	return &Client{
		c:            http.Client{Timeout: time.Second * 30},
		slg:          slg,
		baseGammaURL: gammaURL,
	}
}
```

Change `getUrl` calls: replace the `handlers`-switch usage for gamma with explicit builders on `Client`:

```go
func (c *Client) gammaEventBySlugURL(slug string) string {
	return fmt.Sprintf("%s/events/slug/%s", c.baseGammaURL, url.PathEscape(slug))
}

func (c *Client) gammaEventByIDURL(id string) string {
	return fmt.Sprintf("%s/events/%s", c.baseGammaURL, url.PathEscape(id))
}
```

Update `gamma.go`'s `FetchEventBySlug` to use `c.gammaEventBySlugURL(slug)` and add:

```go
// FetchEventByID fetches a Gamma event by its numeric ID.
func (c *Client) FetchEventByID(ctx context.Context, id string) (*polymarket.Event, error) {
	return makePmGetRequest[polymarket.Event](ctx, c, c.gammaEventByIDURL(id))
}
```

Extend the domain interface in `repository.go`:

```go
type EventProvider interface {
	FetchEventBySlug(ctx context.Context, slug string) (*Event, error)
	FetchEventByID(ctx context.Context, id string) (*Event, error)
}
```

`NewClient`'s return type must become `*Client` (not the interface) so both constructors are symmetric; keep a `var _ polymarket.EventProvider = (*Client)(nil)` compile-time assertion. Update `cmd/sagittarius-mcp/main.go` if the type change requires it (it shouldn't — `NewPmService` takes the interface).

- [ ] **Step 4: Run tests** — `go test ./internal/infrastructure/polymarket/ -v` → PASS; `go build ./...` → OK.

- [ ] **Step 5: Commit** — `git add -A && git commit -m "feat(gamma): add FetchEventByID and testable base URL"`

---

### Task 2: `get_event_by_id` — service, handler, registration

**Files:**
- Modify: `internal/application/polymarket/service.go`, `internal/application/polymarket/dto.go`
- Modify: `internal/interface/mcp/tools/polymarket.go`
- Modify: `internal/interface/mcp/register.go`
- Test: `internal/application/polymarket/service_test.go` (create)

**Interfaces:**
- Produces: `Service.FetchEventByID(ctx context.Context, id string) (*EventIntelligenceContext, error)`; MCP tool `get_event_by_id` with input `FetchEventByIDRequest{ID string `json:"id"`}`.

- [ ] **Step 1: Write the failing test** (mock EventProvider, table-driven):

```go
package polymarket

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	domain "github.com/TheGh0xt/Sagittarius/internal/domain/polymarket"
	"github.com/TheGh0xt/Sagittarius/internal/domain/shared"
)

type stubProvider struct {
	event *domain.Event
	err   error
}

func (s *stubProvider) FetchEventBySlug(ctx context.Context, slug string) (*domain.Event, error) {
	return s.event, s.err
}
func (s *stubProvider) FetchEventByID(ctx context.Context, id string) (*domain.Event, error) {
	return s.event, s.err
}

func TestFetchEventByIDValidation(t *testing.T) {
	svc := NewPmService(&stubProvider{}, slog.Default())
	_, err := svc.FetchEventByID(context.Background(), "")
	var invalid shared.ErrInvalidInput
	if !errors.As(err, &invalid) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
	if _, err := svc.FetchEventByID(context.Background(), "not-a-number"); !errors.As(err, &invalid) {
		t.Fatalf("expected ErrInvalidInput for non-numeric id, got %v", err)
	}
}

func TestFetchEventByIDBuildsContext(t *testing.T) {
	ev := &domain.Event{ID: "42", Title: "T", Volume: 10}
	svc := NewPmService(&stubProvider{event: ev}, slog.Default())
	got, err := svc.FetchEventByID(context.Background(), "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Event.Title != "T" || got.Event.Volume != 10 {
		t.Errorf("context not built from event: %+v", got)
	}
}

func TestFetchEventByIDPropagatesError(t *testing.T) {
	svc := NewPmService(&stubProvider{err: errors.New("boom")}, slog.Default())
	if _, err := svc.FetchEventByID(context.Background(), "42"); err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 2: Run to verify FAIL** — `go test ./internal/application/polymarket/ -v`.

- [ ] **Step 3: Implement.** `dto.go`:

```go
FetchEventByIDRequest struct {
	ID string `json:"id" binding:"required" jsonschema:"numeric Polymarket event ID"`
}
```

`service.go`: extend the `Service` interface and implement (validate non-empty and all-digits via `strconv.Atoi`); on success `return BuildEventIntelligenceContext(event), nil`. Handler in `tools/polymarket.go` mirroring `FetchEventBySlug` exactly (`FetchEventByID` method on `Pmhandler`). Register in `register.go`:

```go
mcp.AddTool(
	s.ms, &mcp.Tool{
		Name:        "get_event_by_id",
		Description: "Get Polymarkets event intelligence by numeric event ID",
	},
	s.ph.FetchEventByID,
)
```

- [ ] **Step 4: Run** — `go test ./... && go build ./...` → PASS.
- [ ] **Step 5: Commit** — `git commit -am "feat(mcp): register get_event_by_id tool"`

---

### Task 3: Market-data domain types + infrastructure (Data/CLOB port)

**Files:**
- Create: `internal/domain/polymarket/market_data.go`
- Modify: `internal/infrastructure/polymarket/client.go` (add `baseDataURL`, `baseClobURL` fields + test constructor params)
- Create: `internal/infrastructure/polymarket/market_data.go`
- Test: `internal/infrastructure/polymarket/market_data_test.go`

**Interfaces (Produces):**

```go
// internal/domain/polymarket/market_data.go
package polymarket

import "context"

// Trade is one executed trade from the Data API (already public/filtered upstream).
type Trade struct {
	Wallet      string  `json:"proxyWallet"`
	Side        string  `json:"side"` // BUY / SELL
	Price       float64 `json:"price"`
	Size        float64 `json:"size"`
	ConditionID string  `json:"conditionId"`
	Timestamp   int64   `json:"timestamp"`
}

type OrderbookLevel struct {
	Price string `json:"price"`
	Size  string `json:"size"`
}

type Orderbook struct {
	AssetID string           `json:"asset_id"`
	Bids    []OrderbookLevel `json:"bids"`
	Asks    []OrderbookLevel `json:"asks"`
}

// MarketDataProvider supplies raw market data for the deterministic Signal
// Engine. Its outputs never reach an LLM directly.
type MarketDataProvider interface {
	// FetchTrades returns recent trades for a market condition ID (Data API).
	FetchTrades(ctx context.Context, conditionID string, limit int) ([]Trade, error)
	// FetchOrderbook returns the current CLOB book for a token ID.
	FetchOrderbook(ctx context.Context, tokenID string) (*Orderbook, error)
}
```

Endpoints: `GET {data}/trades?market={conditionID}&limit={limit}` (Data API, returns a JSON array of trades); `GET {clob}/book?token_id={tokenID}` (CLOB API). `ClobBaseURL = "https://clob.polymarket.com"` — note the real CLOB host is `clob.polymarket.com` (the spec doc's `clob-api.polymarket.com` does not resolve; record this as a design note in the commit).

- [ ] **Step 1: Failing tests** — httptest servers asserting path+query, returning canned JSON for both methods; error-status test for each. Same pattern as Task 1 (write `TestFetchTrades`, `TestFetchOrderbook`, `TestFetchTradesUpstreamError`).
- [ ] **Step 2: Verify FAIL.**
- [ ] **Step 3: Implement** — extend `NewClientWithBaseURLs(slg, gammaURL, dataURL, clobURL string)` (update Task 1 test call sites); implement both fetches with `makePmGetRequest`; add `var _ polymarket.MarketDataProvider = (*Client)(nil)`.
- [ ] **Step 4: `go test ./... && go build ./...`** → PASS.
- [ ] **Step 5: Commit** — `git commit -am "feat(infra): port Data/CLOB market data access to clean architecture"`

---

### Task 4: Pure signal math in `internal/domain/signal`

**Files:**
- Create: `internal/domain/signal/signal.go`
- Test: `internal/domain/signal/signal_test.go`

**Interfaces (Produces):**

```go
package signal

import (
	"math"
	"strconv"
	"time"

	"github.com/TheGh0xt/Sagittarius/internal/domain/polymarket"
)

type WhaleEvent struct {
	Wallet    string    `json:"wallet"`
	Side      string    `json:"side"`
	Price     float64   `json:"price"`
	Size      float64   `json:"size"`
	ValueUSD  float64   `json:"value_usd"`
	Timestamp time.Time `json:"timestamp"`
}

type VolumeSignal struct {
	RecentVolumeUSD   float64 `json:"recent_volume_usd"`
	BaselineVolumeUSD float64 `json:"baseline_volume_usd"`
	VelocityChangePct float64 `json:"velocity_change_pct"`
	IsSpike           bool    `json:"is_spike"` // velocity > +200%
}

type OrderbookSkew struct {
	BidVolumeUSD float64 `json:"bid_volume_usd"`
	AskVolumeUSD float64 `json:"ask_volume_usd"`
	Skew         float64 `json:"skew"`   // (bid-ask)/(bid+ask) in [-1,1]
	Spread       float64 `json:"spread"` // |bestAsk - bestBid|
}

func ComputeWhaleEvents(trades []polymarket.Trade, thresholdUSD float64) []WhaleEvent
func ComputeOrderbookSkew(ob *polymarket.Orderbook) OrderbookSkew
func ComputeVolumeSignal(trades []polymarket.Trade, baselineVolumeUSD float64) VolumeSignal
```

Semantics ported from the prototype `internal/signal/engine.go` (threshold is `>=`, skew guards division by zero, spike is `> 200.0` pct). Port faithfully.

- [ ] **Step 1: Failing tests** — table-driven, real assertions (unlike the prototype's no-op test):
  - Whale: trades of $10k/$25k/$60k with threshold 25k → exactly 2 events, `ValueUSD` = price×size, empty input → nil.
  - Skew: the prototype's documented example — asks [(0.55,100),(0.56,200)], bids [(0.54,150),(0.53,300)] → bidVol 240, askVol 167, skew ≈ (240−167)/407 (use `math.Abs(got-want) < 1e-9`), spread ≈ 0.01; empty book → all zeros.
  - Volume: recent 30k vs baseline 10k → +200% not spike (boundary, `>` not `>=`); 40k vs 10k → +300% spike; zero baseline → velocity 0, no spike.
- [ ] **Step 2: Verify FAIL.**
- [ ] **Step 3: Implement** (port from prototype, adapting field names to the new domain `Trade`/`Orderbook`).
- [ ] **Step 4: `go test ./internal/domain/signal/ -v`** → PASS.
- [ ] **Step 5: Commit** — `git commit -am "feat(signal): port pure signal math to domain layer with real tests"`

---

### Task 5: Signal application service

**Files:**
- Create: `internal/application/signal/service.go`, `internal/application/signal/dto.go`
- Test: `internal/application/signal/service_test.go`

**Interfaces (Produces):**

```go
// dto.go
package signal

type WhaleActivityRequest struct {
	Slug         string  `json:"slug" jsonschema:"Polymarket event slug (path segment after /event/ in the URL)"`
	USDThreshold float64 `json:"usd_threshold,omitempty" jsonschema:"minimum USD notional to classify a trade as a whale event; default 25000"`
	Limit        int     `json:"limit,omitempty" jsonschema:"max trades to scan per market; default 100"`
}

type MarketSnapshotRequest struct {
	Slug string `json:"slug" jsonschema:"Polymarket event slug"`
}

type MarketWhaleActivity struct {
	Question    string              `json:"question"`
	ConditionID string              `json:"condition_id"`
	WhaleEvents []signal.WhaleEvent `json:"whale_events"`
	TotalValueUSD float64           `json:"total_whale_value_usd"`
	BuySellRatio  string            `json:"buy_sell_ratio"` // e.g. "87:13" by USD value
}

type WhaleActivityReport struct {
	EventTitle   string                `json:"event_title"`
	USDThreshold float64               `json:"usd_threshold"`
	Markets      []MarketWhaleActivity `json:"markets"`
}

type MarketSnapshot struct {
	Question       string               `json:"question"`
	ConditionID    string               `json:"condition_id"`
	Probability    float64              `json:"probability"`
	SkewInfo       signal.OrderbookSkew `json:"skew_info"`
	VolumeAnalysis signal.VolumeSignal  `json:"volume_analysis"`
	WhaleCount     int                  `json:"whale_count"`
}

type MarketSnapshotReport struct {
	EventTitle string           `json:"event_title"`
	Timestamp  time.Time        `json:"timestamp"`
	Markets    []MarketSnapshot `json:"markets"`
}

// service.go
type Service interface {
	DetectWhaleActivity(ctx context.Context, req WhaleActivityRequest) (*WhaleActivityReport, error)
	BuildMarketSnapshot(ctx context.Context, slug string) (*MarketSnapshotReport, error)
}

func NewSignalService(ep polymarket.EventProvider, mdp polymarket.MarketDataProvider, slg *slog.Logger) Service
```

**Consumes:** `EventProvider.FetchEventBySlug` (Task 1), `MarketDataProvider` (Task 3), pure funcs (Task 4).

Behavior:
- Both methods resolve the event by slug; empty slug → `shared.ErrInvalidInput`.
- Defaults applied in the service: threshold 25000, limit 100 (also clamp limit to [1,500]).
- Per event market: use `market.ConditionID` for trades. For orderbook, parse the first token from `market.ClobTokenIds` (a JSON-encoded string array, e.g. `"[\"123\",\"456\"]"` → `"123"`); helper `firstClobTokenID(s string) (string, error)` using `encoding/json`.
- Volume baseline for `ComputeVolumeSignal`: `market.Volume24Hr / 24` scaled — keep prototype convention: baseline = `market.VolumeNum / 30.0` when `VolumeNum > 0` else `10000.0` fallback. Document with a comment.
- Snapshot `Probability` = `market.LastTradePrice`. `WhaleCount` = whale events at default 25k threshold over the fetched trades.
- BuySellRatio: share of whale USD value on BUY side vs SELL, formatted `"87:13"`; `"0:0"` when no events.
- A market whose orderbook/trades fetch fails is skipped with a warn log — a partially degraded report is better than a hard error (matches prototype's tolerant behavior).

- [ ] **Step 1: Failing tests** — stub `EventProvider` + stub `MarketDataProvider`; cases: empty slug → ErrInvalidInput; event with 1 market and trades [$30k BUY, $10k SELL, $50k SELL] threshold 25k → 2 whale events, TotalValueUSD 80000, BuySellRatio "38:62" (30000/80000 = 37.5 → round to nearest int, document rounding via `math.Round`); snapshot on same fixtures asserts skew fields and probability; provider error on trades → market skipped, report still returned with 0 markets.
- [ ] **Step 2: Verify FAIL.**
- [ ] **Step 3: Implement.**
- [ ] **Step 4: `go test ./... && go build ./...`** → PASS.
- [ ] **Step 5: Commit** — `git commit -am "feat(signal): add whale activity and market snapshot services"`

---

### Task 6: Signal MCP handlers, registration, wiring

**Files:**
- Create: `internal/interface/mcp/tools/signal.go`
- Modify: `internal/interface/mcp/register.go`, `internal/interface/mcp/server.go`, `cmd/sagittarius-mcp/main.go`

**Interfaces:** handler type `SignalHandler` mirrors `Pmhandler` (constructor `NewSignalHandler(svc signal.Service, slg *slog.Logger)`); methods `DetectWhaleActivity`, `BuildMarketSnapshot` with the exact MCP signature used by `Pmhandler.FetchEventBySlug`. `NewServer` gains the signal service param: `NewServer(ms *mcp.Server, ps polymarket.Service, ss signal.Service, slg *slog.Logger) *Server`. Register:

```go
mcp.AddTool(s.ms, &mcp.Tool{
	Name:        "get_whale_activity",
	Description: "Detect whale-sized trades (notional >= usd_threshold) for every market in a Polymarket event, aggregated per market with buy/sell ratio",
}, s.sh.DetectWhaleActivity)

mcp.AddTool(s.ms, &mcp.Tool{
	Name:        "get_market_snapshot",
	Description: "Unified deterministic state vector per market of a Polymarket event: implied probability, orderbook skew, volume-spike analysis, whale count",
}, s.sh.BuildMarketSnapshot)
```

`main.go`: `client := polymarket.NewClient(logger)` now serves as both providers: `pSvc := pmApp.NewPmService(client, logger)`, `sSvc := sigApp.NewSignalService(client, client, logger)`, `server := smcp.NewServer(ms, pSvc, sSvc, logger)`.

- [ ] **Step 1:** No new unit tests here (handlers are thin marshaling shims over the tested service); the verification is a build + an MCP smoke check.
- [ ] **Step 2: Implement all wiring.**
- [ ] **Step 3: Verify** — `go build ./... && go test ./...` → PASS; then `just build && ./bin/sagittarius-mcp --transport=http --port=8081 &` and confirm `curl -s http://localhost:8081/health` returns `{"status":"ok"}`; kill the server.
- [ ] **Step 4: Commit** — `git commit -am "feat(mcp): register get_whale_activity and get_market_snapshot tools"`

---

### Task 7: Delete superseded prototype packages + docs

**Files:**
- Delete: `internal/polymarket/`, `internal/signal/`, `internal/mcp/` (all three fully superseded now)
- Modify: `README.md` ("What Sagittarius does today" tool list + prototype-packages section), `CLAUDE.md` (registered-tools list + prototype note)

- [ ] **Step 1:** `git rm -r internal/polymarket internal/signal internal/mcp`
- [ ] **Step 2:** `go build ./... && go test ./...` → PASS (nothing in the active tree imports them).
- [ ] **Step 3:** Update `README.md` and `CLAUDE.md`: registered tools are now `get_event_by_slug`, `get_event_by_id`, `get_whale_activity`, `get_market_snapshot`; remove/replace the "prototype packages" sections with a note that the Signal Engine now lives in `internal/domain/signal` + `internal/application/signal`. Also correct the CLOB base URL note (`https://clob.polymarket.com`).
- [ ] **Step 4: Commit** — `git commit -am "refactor: remove prototype packages superseded by clean architecture port"`

## Design Decisions & Assumptions (record in final report)

1. Whale/snapshot tools take an **event slug** (the identifier agents naturally have) and resolve per-market condition IDs internally, rather than taking raw token IDs as the spec sketch suggested.
2. Trades come from the **Data API** (`/trades?market=<conditionId>`), not the CLOB `/trades` endpoint (which requires L2 auth). CLOB is used only for the public `/book` endpoint, at host `clob.polymarket.com`.
3. Volume baseline heuristic retained from the prototype (lifetime volume / 30 days, 10k fallback) — documented inline as an approximation to revisit when price-history tooling lands.
4. Degraded-but-successful reports: individual market fetch failures are logged and skipped, never fail the whole tool call.
