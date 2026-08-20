package polymarket

import (
	"testing"
	"time"
)

// The UI_PRD §6.3 taxonomy is the product's vocabulary; Gamma's tag slugs are
// not. Four of the thirteen do not map by name, and one maps to two tags.
// These were measured against the live Gamma API on 2026-08-20 — see
// docs/superpowers/specs/2026-08-20-automated-report-generation-design.md §3.3.
func TestTagSlugsForCategoryUsesMeasuredMappings(t *testing.T) {
	cases := map[string][]string{
		// Exact matches.
		"Politics":    {"politics"},
		"Elections":   {"elections"},
		"Geopolitics": {"geopolitics"},
		"Crypto":      {"crypto"},
		"AI":          {"ai"},
		"Sports":      {"sports"},
		"Science":     {"science"},
		"Climate":     {"climate"},
		"Health":      {"health"},

		// Approximations. "economics" returns a single open event while
		// "economy" is the live tag; "technology" returns two while "tech"
		// returns a hundred; there is no "entertainment" tag at all.
		"Economics":     {"economy"},
		"Technology":    {"tech"},
		"Entertainment": {"pop-culture"},

		// One category, two tags.
		"Business & Earnings": {"business", "earnings"},
	}

	for category, want := range cases {
		got := tagSlugsForCategory(category)
		if len(got) != len(want) {
			t.Errorf("tagSlugsForCategory(%q) = %v, want %v", category, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("tagSlugsForCategory(%q) = %v, want %v", category, got, want)
				break
			}
		}
	}
}

// An unknown category must not silently become a Gamma query for a tag that
// does not exist — that returns zero events and looks like "nothing is
// moving" rather than "you asked for a category we do not have".
func TestTagSlugsForCategoryRejectsUnknown(t *testing.T) {
	if got := tagSlugsForCategory("Underwater Basket Weaving"); got != nil {
		t.Errorf("unknown category returned %v, want nil", got)
	}
}

// "culture" was tested against the live API and returns zero open events. It
// is a plausible-looking guess for Entertainment and must never be used.
func TestCultureIsNotUsedForEntertainment(t *testing.T) {
	for _, slug := range tagSlugsForCategory("Entertainment") {
		if slug == "culture" {
			t.Error("Entertainment mapped to 'culture', which returns no open events")
		}
	}
}

// The taxonomy is fixed at thirteen categories by UI_PRD §6.3. Callers that
// pass no categories analyse all of them, so this list is the default universe.
func TestAllCategoriesCoversTheProductTaxonomy(t *testing.T) {
	all := AllCategories()
	if len(all) != 13 {
		t.Fatalf("AllCategories() has %d entries, want 13", len(all))
	}
	for _, category := range all {
		if tagSlugsForCategory(category) == nil {
			t.Errorf("AllCategories() includes %q, which has no tag mapping", category)
		}
	}
}

// A market that resolves before the evaluation engine scores it cannot be
// scored meaningfully: it settles to 0 or 1, which evaluate_report reads as a
// large extended move and marks CONFIRMED. Without this filter the corpus
// fills with same-day esports matches and manufactures a flattering,
// meaningless hit rate.
func TestIsScoreableExcludesMarketsResolvingBeforeTheHorizon(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name    string
		endDate time.Time
		want    bool
	}{
		{"resolves today", now.Add(6 * time.Hour), false},
		{"resolves inside the 48h window", now.Add(36 * time.Hour), false},
		{"resolves just after 48h but inside the margin", now.Add(3 * 24 * time.Hour), false},
		{"resolves exactly at the cutoff", now.Add(7 * 24 * time.Hour), true},
		{"resolves well beyond", now.Add(70 * 24 * time.Hour), true},
		{"already ended", now.Add(-24 * time.Hour), false},
	}

	for _, tc := range cases {
		if got := isScoreable(tc.endDate, now, 7); got != tc.want {
			t.Errorf("%s: isScoreable(%v) = %v, want %v", tc.name, tc.endDate, got, tc.want)
		}
	}
}

// The cutoff is 7 days rather than 48 hours on purpose: a market resolving
// between the 24h and 48h checkpoints would skew the later ones while passing
// a naive 48h filter.
func TestIsScoreableCutoffExceedsTheCanonicalHorizon(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	justPastCanonical := now.Add(49 * time.Hour)

	if isScoreable(justPastCanonical, now, 7) {
		t.Error("a market resolving just after the 48h checkpoint was accepted; " +
			"it would skew the 48h score without a margin")
	}
}

// UI_PRD §6.4: "ranked by movement, not popularity". Volume must not decide
// order — that is what surfaces liquid-but-static markets.
func TestRankByMovementOrdersByAbsoluteChangeNotVolume(t *testing.T) {
	markets := []MovingMarket{
		{Slug: "quiet-whale", Change24h: 0.01, Volume24h: 9_000_000},
		{Slug: "big-mover", Change24h: 0.22, Volume24h: 1_000},
		{Slug: "middling", Change24h: 0.09, Volume24h: 500_000},
	}

	rankByMovement(markets)

	want := []string{"big-mover", "middling", "quiet-whale"}
	for i, slug := range want {
		if markets[i].Slug != slug {
			t.Fatalf("rank %d = %q, want %q (full order: %v)",
				i, markets[i].Slug, slug, slugsOf(markets))
		}
	}
}

// A collapse is as much of a move as a spike, and PMIE exists to explain
// either. Ranking on the signed value would bury every downward move.
func TestRankByMovementTreatsFallsAsEquallyInteresting(t *testing.T) {
	markets := []MovingMarket{
		{Slug: "small-rise", Change24h: 0.04},
		{Slug: "large-fall", Change24h: -0.31},
	}

	rankByMovement(markets)

	if markets[0].Slug != "large-fall" {
		t.Errorf("a -0.31 move ranked below a +0.04 move: %v", slugsOf(markets))
	}
}

func slugsOf(markets []MovingMarket) []string {
	out := make([]string, len(markets))
	for i, m := range markets {
		out[i] = m.Slug
	}
	return out
}
