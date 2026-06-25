package polymarket

import (
	"context"
)

func (c *Client) FetchEventBySlug(ctx context.Context, slug string) (*Event, error) {
	url := getUrl("/events/slug/%s", slug)

	return makePmGetRequest[Event](ctx, c, url)
}
