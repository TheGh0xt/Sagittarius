package polymarket

import (
	"math"
	"sort"
	"time"
)

// Market discovery for the personalised feed (UI_PRD §6.4) and the automated
// report generator (ROADMAP 4.12). Both need the same thing: markets in a set
// of categories that are currently moving and will still be open when the
// evaluation engine comes to score them.
//
// Ranking is by movement, not popularity. UI_PRD §6.4 is explicit about this,
// and it matters more than it looks: ranking by volume surfaces same-day
// esports matches, which resolve long before T+48h and score CONFIRMED
// trivially, inflating the accuracy record with markets nobody asked about.

// categoryTags maps the fixed UI_PRD §6.3 taxonomy onto Gamma's tag slugs.
//
// Gamma's tag vocabulary is a long tail of granular tags — individual players,
// teams and people — rather than a clean taxonomy, so four of these do not map
// by name. Every entry was verified against the live API on 2026-08-20 to
// return open events:
//
//   - Economics uses "economy"; "economics" returns a single open event.
//   - Technology uses "tech"; "technology" returns two, "tech" returns 100+.
//   - Entertainment uses "pop-culture"; there is no "entertainment" tag, and
//     "culture" returns zero open events.
//   - Business & Earnings is the union of two tags, not one.
//
// These four are approximations recorded for correction, not discoveries.
var categoryTags = map[string][]string{
	"Politics":            {"politics"},
	"Elections":           {"elections"},
	"Geopolitics":         {"geopolitics"},
	"Economics":           {"economy"},
	"Crypto":              {"crypto"},
	"Business & Earnings": {"business", "earnings"},
	"Technology":          {"tech"},
	"AI":                  {"ai"},
	"Sports":              {"sports"},
	"Entertainment":       {"pop-culture"},
	"Science":             {"science"},
	"Climate":             {"climate"},
	"Health":              {"health"},
}

// orderedCategories fixes the iteration order of the taxonomy.
//
// Map iteration in Go is deliberately randomised, and the generator
// round-robins across categories to stop one dominating the corpus. That is
// only meaningful against a stable order.
var orderedCategories = []string{
	"Politics",
	"Elections",
	"Geopolitics",
	"Economics",
	"Crypto",
	"Business & Earnings",
	"Technology",
	"AI",
	"Sports",
	"Entertainment",
	"Science",
	"Climate",
	"Health",
}

// AllCategories returns the full UI_PRD §6.3 taxonomy in a stable order.
//
// This is the default universe: a caller that names no categories gets all of
// them, which is what the generator does while no users have onboarded yet.
func AllCategories() []string {
	out := make([]string, len(orderedCategories))
	copy(out, orderedCategories)
	return out
}

// tagSlugsForCategory returns the Gamma tag slugs for a product category, or
// nil if the category is not in the taxonomy.
//
// nil rather than a passthrough guess: querying Gamma for a tag that does not
// exist returns zero events, which is indistinguishable from "nothing in this
// category is moving" and would hide the caller's mistake.
func tagSlugsForCategory(category string) []string {
	slugs, ok := categoryTags[category]
	if !ok {
		return nil
	}
	out := make([]string, len(slugs))
	copy(out, slugs)
	return out
}

// MovingMarket is one candidate event, condensed for a caller that has to
// choose between candidates.
//
// Deliberately not the full EventIntelligenceContext: this is a screen, and
// the caller's next step for anything it picks is get_event_by_slug, which
// returns the whole picture. Repeating market detail here would spend context
// on data that is about to be fetched properly.
type MovingMarket struct {
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	Category string `json:"category"`

	Probability float64 `json:"probability"`
	Change1h    float64 `json:"change_1h"`
	Change24h   float64 `json:"change_24h"`
	Volume24h   float64 `json:"volume_24h"`

	EndDate string `json:"end_date"`
}

// isScoreable reports whether a market will still be open when the evaluation
// engine scores it.
//
// A market that resolves inside the evaluation window settles to 0 or 1, which
// the confidence matrix reads as a large extended move and marks CONFIRMED.
// Admitting those would fill the accuracy record with markets that could not
// have been wrong.
//
// minDays is deliberately larger than the 48h canonical horizon: a market
// resolving between the 24h and 48h checkpoints would skew the later ones
// while passing a naive 48h test.
func isScoreable(endDate, now time.Time, minDays int) bool {
	cutoff := now.Add(time.Duration(minDays) * 24 * time.Hour)
	return !endDate.Before(cutoff)
}

// rankByMovement sorts candidates by how far they have moved in 24h, largest
// first, in place.
//
// Absolute value, because a collapse is as much of a move as a spike and PMIE
// exists to explain either; ranking on the signed change would bury every
// downward move. Volume is carried for display but never decides order —
// UI_PRD §6.4 specifies "ranked by movement, not popularity", and ranking by
// volume is what surfaces liquid-but-static markets.
func rankByMovement(markets []MovingMarket) {
	sort.SliceStable(markets, func(i, j int) bool {
		return math.Abs(markets[i].Change24h) > math.Abs(markets[j].Change24h)
	})
}

// settledBand is how close to 0 or 1 a probability must be before the market
// is treated as decided.
//
// A multi-market event stays open while its individual sub-markets resolve —
// "Who will Trump endorse" ran to 2026-11-04 with one candidate's market
// already at 0.999. Those produce the largest 24h "moves" in the whole feed
// and every one of them is a settlement rather than a price movement.
//
// Surfacing them is worse than useless. A settled market cannot move again, so
// explaining "why it moved" means explaining a resolution; and at T+48h it is
// still at 0.999, which the confidence matrix scores CONFIRMED for free. Left
// in, they would dominate both the feed and the accuracy record.
const settledBand = 0.02

// isTradeable reports whether a market still has room to move.
func isTradeable(probability float64) bool {
	return probability > settledBand && probability < 1-settledBand
}
