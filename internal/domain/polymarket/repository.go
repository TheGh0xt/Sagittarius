package polymarket

import (
	"context"
)

type (
	EventProvider interface {
		FetchEventBySlug(ctx context.Context, slug string) (*Event, error)
		FetchEventByID(ctx context.Context, id string) (*Event, error)
	}

	// MarketDataProvider supplies raw market data for the deterministic Signal
	// Engine. Its outputs never reach an LLM directly — they are condensed by the
	// signal application layer first.
	MarketDataProvider interface {
		// FetchTrades returns recent trades for a market condition ID (Data API).
		FetchTrades(ctx context.Context, conditionID string, limit int) ([]Trade, error)
		// FetchOrderbook returns the current CLOB book for a token ID.
		FetchOrderbook(ctx context.Context, tokenID string) (*Orderbook, error)
	}
)
