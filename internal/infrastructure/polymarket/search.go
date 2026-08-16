package polymarket

import (
	"context"

	"github.com/TheGh0xt/Sagittarius/internal/domain/polymarket"
)

// SearchEvents finds live events matching free text.
//
// events_status=active is not optional. Without it Gamma happily returns
// settled events from previous years, and a search for "ballon dor" leads
// with the 2025 winner market that closed months ago. An agent picking the
// top hit would then analyse a market whose price can never move again.
func (c *Client) SearchEvents(
	ctx context.Context, query string, limit int,
) (*polymarket.SearchResults, error) {
	return makePmGetRequest[polymarket.SearchResults](
		ctx, c, c.gammaSearchURL(query, limit),
	)
}
