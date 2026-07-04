package polymarket

import (
	"context"

	"github.com/TheGh0xt/Sagittarius/internal/domain/polymarket"
)

// FetchEventBySlug fetches a Gamma event by its URL slug.
func (c *Client) FetchEventBySlug(ctx context.Context, slug string) (*polymarket.Event, error) {
	return makePmGetRequest[polymarket.Event](ctx, c, c.gammaEventBySlugURL(slug))
}

// FetchEventByID fetches a Gamma event by its numeric ID.
func (c *Client) FetchEventByID(ctx context.Context, id string) (*polymarket.Event, error) {
	return makePmGetRequest[polymarket.Event](ctx, c, c.gammaEventByIDURL(id))
}
