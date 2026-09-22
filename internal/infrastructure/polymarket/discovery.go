package polymarket

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/TheGh0xt/Sagittarius/internal/domain/polymarket"
)

// gammaEventsByTagURL builds a query for open events carrying one tag.
//
// closed=false and archived=false are load-bearing for the same reason
// events_status=active is on search: without them Gamma returns settled events
// alongside live ones, and a feed of "markets currently moving" would lead
// with markets whose price can never move again.
//
// They are not sufficient, though. Measured against the live API on
// 2026-09-22, `tag_slug=crypto&closed=false&archived=false&limit=20` returns
// events in **id-ascending order — oldest first** — and four of the twenty had
// end dates 265 days in the past despite closed=false. An earlier version of
// this function asked for no ordering at all, on the stated premise that Gamma
// "returns them unordered for our purposes". That premise was wrong, and it is
// the reason the feed was full of stale, zombie markets: with a limit applied
// to an oldest-first list, the biggest mover is almost never inside the window,
// however wide the window is.
//
// So two parameters now do the selecting, and both are about *which candidates
// survive the limit* — not about final rank:
//
//   - order=volume24hr&ascending=false picks the candidates most likely to be
//     worth looking at. This does NOT conflict with UI_PRD §6.4's "rank by
//     movement, not popularity": the caller still ranks by movement
//     (rankByMovement). The choice here is only between volume and event age as
//     the sampling heuristic, and age is indefensible.
//   - end_date_min drops events resolving too soon to be scoreable upstream,
//     instead of fetching them and discarding them after the limit has already
//     cost us the slot.
func (c *Client) gammaEventsByTagURL(
	tagSlug string, limit int, endDateMin time.Time,
) string {
	params := url.Values{}
	params.Set("tag_slug", tagSlug)
	params.Set("closed", "false")
	params.Set("archived", "false")
	params.Set("limit", strconv.Itoa(limit))
	params.Set("order", "volume24hr")
	params.Set("ascending", "false")
	if !endDateMin.IsZero() {
		params.Set("end_date_min", endDateMin.UTC().Format(time.RFC3339))
	}
	return fmt.Sprintf("%s/events?%s", c.baseGammaURL, params.Encode())
}

// FetchEventsByTag returns open Gamma events carrying the given tag slug,
// resolving no sooner than endDateMin, most-traded first.
func (c *Client) FetchEventsByTag(
	ctx context.Context, tagSlug string, limit int, endDateMin time.Time,
) ([]polymarket.Event, error) {
	events, err := makePmGetRequest[[]polymarket.Event](
		ctx, c, c.gammaEventsByTagURL(tagSlug, limit, endDateMin),
	)
	if err != nil {
		return nil, err
	}
	return *events, nil
}
