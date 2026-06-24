package polymarket

import (
	"context"
	"fmt"
	"net/url"
)

type Trade struct {
	ID        string  `json:"id"`
	Price     float64 `json:"price,string"` // Can be string in JSON
	Size      float64 `json:"size,string"`  // Can be string in JSON
	Side      string  `json:"side"`         // BUY/SELL
	Timestamp int64   `json:"timestamp"`    // Unix epoch or custom representation
}

type Holder struct {
	User       string  `json:"user"`
	Balance    float64 `json:"balance,string"`
	Percentage float64 `json:"percentage"`
}

func (c *Client) GetTrades(ctx context.Context, tokenID string, limit int) ([]Trade, error) {
	if limit <= 0 {
		limit = 50
	}
	// The CLOB or Data API trades endpoint
	u := fmt.Sprintf("%s/trades?market=%s&limit=%d", ClobBaseURL, url.QueryEscape(tokenID), limit)

	// Create struct representation for trades response
	var trades []Trade
	if err := c.getJSON(ctx, u, &trades, false); err != nil {
		// Fallback try: some interfaces return as object { "trades": [...] }
		var wrapper struct {
			Trades []Trade `json:"trades"`
		}
		if err2 := c.getJSON(ctx, u, &wrapper, false); err2 == nil {
			return wrapper.Trades, nil
		}
		return nil, err
	}
	return trades, nil
}

func (c *Client) GetHolders(ctx context.Context, marketID string) ([]Holder, error) {
	// Query Data API for holders of a specific market ID (condition_id)
	u := fmt.Sprintf("%s/holders?market=%s", DataBaseURL, url.QueryEscape(marketID))

	var holders []Holder
	if err := c.getJSON(ctx, u, &holders, false); err != nil {
		// Let's also support object wrapping response
		var wrapper struct {
			Holders []Holder `json:"holders"`
		}
		if err2 := c.getJSON(ctx, u, &wrapper, false); err2 == nil {
			return wrapper.Holders, nil
		}
		return nil, err
	}
	return holders, nil
}
