package polymarket

import (
	"context"

	"github.com/TheGh0xt/Sagittarius/internal/domain/polymarket"
)

func (c *Client) FetchEventBySlug(ctx context.Context, slug string) (*polymarket.Event, error) {
	url := getUrl("/events/slug/%s", slug)

	return makePmGetRequest[polymarket.Event](ctx, c, url)
}
