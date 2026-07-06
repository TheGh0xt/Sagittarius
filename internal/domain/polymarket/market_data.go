package polymarket

import "context"

// Trade is one executed trade from the Data API.
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
// Engine. Its outputs never reach an LLM directly — they are condensed by the
// signal application layer first.
type MarketDataProvider interface {
	// FetchTrades returns recent trades for a market condition ID (Data API).
	FetchTrades(ctx context.Context, conditionID string, limit int) ([]Trade, error)
	// FetchOrderbook returns the current CLOB book for a token ID.
	FetchOrderbook(ctx context.Context, tokenID string) (*Orderbook, error)
}
