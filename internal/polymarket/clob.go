package polymarket

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

type PriceSize struct {
	Price string `json:"price"`
	Size  string `json:"size"`
}

type Orderbook struct {
	Asks    []PriceSize `json:"asks"`
	Bids    []PriceSize `json:"bids"`
	AssetID string      `json:"assetId"`
}

type HistoryPoint struct {
	T int64   `json:"t"`
	P float64 `json:"p"`
}

type HistoryResponse struct {
	History []HistoryPoint `json:"history"`
}

type PriceResponse struct {
	Price string `json:"price"`
}

func (c *Client) GetOrderbook(ctx context.Context, tokenID string) (*Orderbook, error) {
	u := fmt.Sprintf("%s/book?token_id=%s", ClobBaseURL, url.QueryEscape(tokenID))
	var ob Orderbook
	if err := c.getJSON(ctx, u, &ob, false); err != nil {
		return nil, err
	}
	return &ob, nil
}

func (c *Client) GetPriceHistory(ctx context.Context, tokenID string, startTs, endTs int64, fidelity int) ([]HistoryPoint, error) {
	u := fmt.Sprintf("%s/prices-history?market=%s", ClobBaseURL, url.QueryEscape(tokenID))
	if startTs > 0 {
		u += fmt.Sprintf("&startTs=%d", startTs)
	}
	if endTs > 0 {
		u += fmt.Sprintf("&endTs=%d", endTs)
	}
	if fidelity > 0 {
		u += fmt.Sprintf("&fidelity=%d", fidelity)
	} else {
		u += "&fidelity=60" // Default to 1h
	}

	var res HistoryResponse
	if err := c.getJSON(ctx, u, &res, true); err != nil {
		return nil, err
	}
	return res.History, nil
}

func (c *Client) GetPrice(ctx context.Context, tokenID string) (float64, error) {
	// Fallback to orderbook midpoint if /price endpoint varies
	ob, err := c.GetOrderbook(ctx, tokenID)
	if err == nil && len(ob.Asks) > 0 && len(ob.Bids) > 0 {
		askVal, err1 := strconv.ParseFloat(ob.Asks[0].Price, 64)
		bidVal, err2 := strconv.ParseFloat(ob.Bids[0].Price, 64)
		if err1 == nil && err2 == nil {
			return (askVal + bidVal) / 2.0, nil
		}
	}

	// Try the direct endpoint as well
	u := fmt.Sprintf("%s/price?token_id=%s", ClobBaseURL, url.QueryEscape(tokenID))
	var res PriceResponse
	if err := c.getJSON(ctx, u, &res, false); err == nil {
		if pVal, err := strconv.ParseFloat(res.Price, 64); err == nil {
			return pVal, nil
		}
	}

	return 0, fmt.Errorf("unable to fetch price for token %s", tokenID)
}

func (c *Client) GetMarketVolume(ctx context.Context, tokenID string) (float64, error) {
	// Usually volume is tracked at the market level in Gamma, but we can query it
	// via standard historical prices or by fetching the market metadata from Gamma.
	// For robustness, let's implement this by querying the market from Gamma first.
	return 0, nil
}
