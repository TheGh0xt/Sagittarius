package polymarket

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/TheGh0xt/Sagittarius/internal/domain/polymarket"
)

// gammaEventsByTagURL builds a query for open events carrying one tag.
//
// closed=false and archived=false are load-bearing for the same reason
// events_status=active is on search: without them Gamma returns settled events
// alongside live ones, and a feed of "markets currently moving" would lead
// with markets whose price can never move again.
//
// Ordering is deliberately NOT requested from Gamma. Its order parameter sorts
// by volume, and UI_PRD §6.4 specifies ranking by movement rather than
// popularity — so the caller ranks, and asking Gamma to pre-sort would only
// bias which events survive the limit.
func (c *Client) gammaEventsByTagURL(tagSlug string, limit int) string {
	params := url.Values{}
	params.Set("tag_slug", tagSlug)
	params.Set("closed", "false")
	params.Set("archived", "false")
	params.Set("limit", strconv.Itoa(limit))
	return fmt.Sprintf("%s/events?%s", c.baseGammaURL, params.Encode())
}

// FetchEventsByTag returns open Gamma events carrying the given tag slug.
func (c *Client) FetchEventsByTag(
	ctx context.Context, tagSlug string, limit int,
) ([]polymarket.Event, error) {
	events, err := makePmGetRequest[[]polymarket.Event](
		ctx, c, c.gammaEventsByTagURL(tagSlug, limit),
	)
	if err != nil {
		return nil, err
	}
	return *events, nil
}
