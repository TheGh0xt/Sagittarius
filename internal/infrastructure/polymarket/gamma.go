package polymarket

import (
	"context"
	"fmt"

	"github.com/TheGh0xt/Sagittarius/internal/domain/polymarket"
)

func (c *Client) FetchEventBySlug(ctx context.Context, slug string) (*polymarket.Event, error) {
	url := getUrl(GammaHandler, slug)

	if url == "" {
		return nil, fmt.Errorf("invalid url: %s", url)
	}

	return makePmGetRequest[polymarket.Event](ctx, c, url)
}
