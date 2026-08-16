package polymarket

import (
	"context"
	"strings"

	"github.com/TheGh0xt/Sagittarius/internal/domain/shared"
)

const (
	defaultSearchLimit = 5
	maxSearchLimit     = 20
)

type (
	// SearchMarketsRequest is the tool's input.
	SearchMarketsRequest struct {
		Query string `json:"query" binding:"required" jsonschema:"free-text description of the market or event to find, for example 'ballon dor' or 'who wins the world cup'"`
		Limit int    `json:"limit,omitempty" jsonschema:"maximum results to return; defaults to 5, capped at 20"`
	}

	// SearchMatch is one candidate event, condensed.
	//
	// Enough to choose between candidates and nothing more. The caller's next
	// step is get_event_by_slug, which returns the full picture, so repeating
	// market detail here would spend context on data about to be fetched
	// properly.
	SearchMatch struct {
		Slug  string `json:"slug"`
		Title string `json:"title"`

		Volume    float64 `json:"volume"`
		Volume24h float64 `json:"volume_24h"`
		Liquidity float64 `json:"liquidity"`

		MarketCount int      `json:"market_count"`
		Tags        []string `json:"tags"`
		EndDate     string   `json:"end_date"`
	}

	SearchMarketsResponse struct {
		Query   string        `json:"query"`
		Matches []SearchMatch `json:"matches"`
	}
)

// SearchMarkets resolves free text to candidate Polymarket events.
//
// This is what lets a question like "who will win the ballon dor?" reach a
// market at all. Without it the only way in is a slug the user already knows,
// which is fine for a URL paste and useless for a question.
func (pms *pmService) SearchMarkets(
	ctx context.Context, req SearchMarketsRequest,
) (*SearchMarketsResponse, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return nil, shared.ErrInvalidInput{Field: "query", Message: "query cannot be empty"}
	}

	limit := req.Limit
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	if limit > maxSearchLimit {
		limit = maxSearchLimit
	}

	// People ask questions; Gamma matches phrases. Sending the raw question
	// surfaces every other "who will win ..." market and buries the one that
	// was meant.
	results, err := pms.sp.SearchEvents(ctx, normaliseQuery(query), limit)
	if err != nil {
		pms.slg.Error("Sagittarius", "state", "search failed", "error", err)
		return nil, err
	}

	response := &SearchMarketsResponse{Query: query, Matches: []SearchMatch{}}
	if results == nil {
		return response, nil
	}

	for _, event := range results.Events {
		// Gamma's status filter is applied server-side, but a closed or
		// archived event slipping through would send the agent to a market
		// whose price can never move again. Cheap to re-check.
		if event.Closed || event.Archived || !event.Active {
			continue
		}

		tags := make([]string, 0, len(event.Tags))
		for _, tag := range event.Tags {
			tags = append(tags, tag.Label)
		}

		response.Matches = append(response.Matches, SearchMatch{
			Slug:        event.Slug,
			Title:       strings.TrimSpace(event.Title),
			Volume:      event.Volume,
			Volume24h:   event.Volume24h,
			Liquidity:   event.Liquidity,
			MarketCount: len(event.Markets),
			Tags:        tags,
			EndDate:     event.EndDate,
		})
	}

	return response, nil
}
