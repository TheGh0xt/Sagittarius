package polymarket

import "context"

// SearchResults is Gamma's public-search response.
//
// Deliberately a separate, lean type rather than a reuse of Event: search
// returns a different and partial projection of an event, and decoding it as
// a full Event would either fail outright or silently produce zero values
// that look like real data.
type SearchResults struct {
	Events []SearchedEvent `json:"events"`
}

// SearchedEvent is one event as returned by search.
//
// Only the fields needed to choose between candidates are decoded. Gamma
// returns far more, and every extra field is another chance for a type
// surprise on a payload nobody is watching.
type SearchedEvent struct {
	ID    string `json:"id"`
	Slug  string `json:"slug"`
	Title string `json:"title"`

	Active   bool `json:"active"`
	Closed   bool `json:"closed"`
	Archived bool `json:"archived"`

	// Every Gamma number is a float64. Fields that look integral arrive
	// fractional, and one wrong type fails the whole decode.
	Volume    float64 `json:"volume"`
	Volume24h float64 `json:"volume24hr"`
	Liquidity float64 `json:"liquidity"`

	EndDate string `json:"endDate"`

	Tags []SearchedTag `json:"tags"`

	Markets []SearchedMarket `json:"markets"`
}

type SearchedTag struct {
	Label string `json:"label"`
	Slug  string `json:"slug"`
}

type SearchedMarket struct {
	ConditionID string `json:"conditionId"`
	Question    string `json:"question"`
}

// SearchProvider finds events matching free text.
//
// Separate from EventProvider because the two answer different questions:
// EventProvider fetches an event you can already name, SearchProvider works
// out which event a person meant.
type SearchProvider interface {
	SearchEvents(ctx context.Context, query string, limit int) (*SearchResults, error)
}
