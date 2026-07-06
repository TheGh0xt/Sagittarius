package polymarket

import (
	"context"
	"fmt"
	"net/url"

	"github.com/TheGh0xt/Sagittarius/internal/domain/polymarket"
)

var _ polymarket.MarketDataProvider = (*Client)(nil)

// FetchTrades returns recent trades for a market condition ID via the public
// Data API. The CLOB /trades endpoint requires L2 auth, so it is not used.
func (c *Client) FetchTrades(ctx context.Context, conditionID string, limit int) ([]polymarket.Trade, error) {
	u := fmt.Sprintf("%s/trades?market=%s&limit=%d", c.baseDataURL, url.QueryEscape(conditionID), limit)
	trades, err := makePmGetRequest[[]polymarket.Trade](ctx, c, u)
	if err != nil {
		return nil, err
	}
	return *trades, nil
}

// FetchOrderbook returns the current CLOB book snapshot for a token ID.
func (c *Client) FetchOrderbook(ctx context.Context, tokenID string) (*polymarket.Orderbook, error) {
	u := fmt.Sprintf("%s/book?token_id=%s", c.baseClobURL, url.QueryEscape(tokenID))
	return makePmGetRequest[polymarket.Orderbook](ctx, c, u)
}
