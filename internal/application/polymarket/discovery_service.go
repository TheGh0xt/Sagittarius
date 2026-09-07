package polymarket

import (
	"context"
	"math"
	"time"

	domain "github.com/TheGh0xt/Sagittarius/internal/domain/polymarket"
	"github.com/TheGh0xt/Sagittarius/internal/domain/shared"
)

const (
	defaultMovingLimit = 20
	maxMovingLimit     = 50

	// How many events to pull per tag before ranking. Gamma returns them
	// unordered for our purposes — we deliberately do not ask it to sort,
	// because its only sort is by volume and that is the wrong axis — so the
	// window has to be wide enough that the biggest mover is inside it.
	eventsPerTag = 40

	// Days a market must still have left before it is worth analysing. Larger
	// than the 48h canonical horizon on purpose; see isScoreable.
	defaultMinDaysToResolution = 7
)

type (
	// GetMovingMarketsRequest is the tool's input.
	GetMovingMarketsRequest struct {
		Categories          []string `json:"categories,omitempty" jsonschema:"product categories to search, for example 'Politics' or 'Crypto'; defaults to all thirteen"`
		Limit               int      `json:"limit,omitempty" jsonschema:"maximum markets to return; defaults to 20, capped at 50"`
		MinDaysToResolution int      `json:"min_days_to_resolution,omitempty" jsonschema:"exclude markets resolving sooner than this many days; defaults to 7"`
	}

	GetMovingMarketsResponse struct {
		Categories []string       `json:"categories"`
		Markets    []MovingMarket `json:"markets"`
	}
)

// GetMovingMarkets returns open markets in the given categories that have
// moved most in the last 24 hours.
//
// This is the shared backing for the personalised feed (UI_PRD §6.4) and the
// automated report generator (ROADMAP 4.12) — the same question, differing
// only in whose categories are asked for and what happens to the answer.
func (pms *pmService) GetMovingMarkets(
	ctx context.Context, req GetMovingMarketsRequest,
) (*GetMovingMarketsResponse, error) {
	if pms.dp == nil {
		return nil, shared.ErrInternalServerError{
			Message: "market discovery is not configured on this server",
		}
	}

	categories := req.Categories
	if len(categories) == 0 {
		categories = AllCategories()
	}

	// Validate every category before doing any I/O, so a typo fails fast and
	// loudly rather than quietly returning a short list.
	for _, category := range categories {
		if tagSlugsForCategory(category) == nil {
			return nil, shared.ErrInvalidInput{
				Field:   "categories",
				Message: "unknown category: " + category,
			}
		}
	}

	limit := req.Limit
	if limit <= 0 {
		limit = defaultMovingLimit
	}
	if limit > maxMovingLimit {
		limit = maxMovingLimit
	}

	minDays := req.MinDaysToResolution
	if minDays <= 0 {
		minDays = defaultMinDaysToResolution
	}

	now := time.Now().UTC()
	perCategory := make([][]MovingMarket, 0, len(categories))

	for _, category := range categories {
		found := pms.movingMarketsIn(ctx, category, now, minDays)
		rankByMovement(found)
		perCategory = append(perCategory, found)
	}

	return &GetMovingMarketsResponse{
		Categories: categories,
		Markets:    interleave(perCategory, limit),
	}, nil
}

// movingMarketsIn collects the scoreable markets of one category.
//
// A failing tag is logged and skipped rather than returned as an error: one
// category being unavailable upstream must not cost the caller every other
// category. Degraded-but-successful, per the standing convention — and visibly
// so, because the failure is logged rather than swallowed.
func (pms *pmService) movingMarketsIn(
	ctx context.Context, category string, now time.Time, minDays int,
) []MovingMarket {
	var out []MovingMarket

	for _, tagSlug := range tagSlugsForCategory(category) {
		events, err := pms.dp.FetchEventsByTag(ctx, tagSlug, eventsPerTag)
		if err != nil {
			pms.slg.Error("discovery failed for tag; skipping",
				"category", category, "tag", tagSlug, "err", err)
			continue
		}

		for i := range events {
			if market, ok := movingMarketFrom(&events[i], category, now, minDays); ok {
				out = append(out, market)
			}
		}
	}

	return out
}

// movingMarketFrom condenses one Gamma event into a candidate, reporting false
// if the event is not worth analysing.
//
// An event holds many markets — the Ballon d'Or event carries 89 — so "how far
// did this event move" is the largest absolute 24h move among them. That is
// the market a reader would actually be asking about.
func movingMarketFrom(
	event *domain.Event, category string, now time.Time, minDays int,
) (MovingMarket, bool) {
	if !isScoreable(event.EndDate, now, minDays) {
		return MovingMarket{}, false
	}

	lead := -1
	for i := range event.Markets {
		if !isTradeable(event.Markets[i].LastTradePrice) {
			continue
		}
		if event.Markets[i].Closed || (!event.Markets[i].Active && event.Markets[i].Approved) {
			continue
		}
		if lead < 0 || math.Abs(event.Markets[i].OneDayPriceChange) >
			math.Abs(event.Markets[lead].OneDayPriceChange) {
			lead = i
		}
	}
	if lead < 0 {
		return MovingMarket{}, false
	}
	market := event.Markets[lead]

	return MovingMarket{
		Slug:        event.Slug,
		Title:       event.Title,
		Category:    category,
		Probability: market.LastTradePrice,
		Change1h:    float64(market.OneHourPriceChange),
		Change24h:   market.OneDayPriceChange,
		Volume24h:   event.Volume24Hr,
		EndDate:     event.EndDate.Format(time.RFC3339),
	}, true
}

// interleave takes from each category in turn until it has `limit` markets.
//
// Round-robin rather than a single global ranking: one busy category would
// otherwise take every slot, and a corpus dominated by whichever domain
// happened to be volatile makes the per-category accuracy record (4.11)
// unsliceable. Each category's list arrives already ranked by movement, so
// this still takes each category's biggest mover first.
func interleave(groups [][]MovingMarket, limit int) []MovingMarket {
	out := make([]MovingMarket, 0, limit)

	for round := 0; len(out) < limit; round++ {
		progressed := false
		for _, group := range groups {
			if round >= len(group) {
				continue
			}
			out = append(out, group[round])
			progressed = true
			if len(out) == limit {
				return out
			}
		}
		if !progressed {
			break
		}
	}

	return out
}
