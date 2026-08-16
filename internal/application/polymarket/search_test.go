package polymarket

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	domain "github.com/TheGh0xt/Sagittarius/internal/domain/polymarket"
	"github.com/TheGh0xt/Sagittarius/internal/domain/shared"
)

// Gamma matches phrases, so a raw question scores against every other
// question-shaped market. "who will win the ballon dor?" returned a Rhode
// Island mayoral race and Dancing With The Stars, with the Ballon d'Or market
// absent entirely. Searching the distinctive terms returns it first.
func TestNormaliseQueryStripsQuestionWords(t *testing.T) {
	cases := map[string]string{
		// "win" survives on purpose: market titles contain it, and a live
		// search for this exact string returns ballon-dor-winner-2026 first.
		"who will win the ballon dor?":    "win ballon dor",
		"why is world-cup-winner moving?": "world-cup-winner",
		"What is happening to Bitcoin":    "bitcoin",
		"will bitcoin hit 150k":           "bitcoin hit 150k",
		"ballon dor":                      "ballon dor",
		"Tell me about the election":      "election",
	}

	for input, want := range cases {
		if got := normaliseQuery(input); got != want {
			t.Errorf("normaliseQuery(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormaliseQueryKeepsSlugsIntact(t *testing.T) {
	// A pasted slug is one of the most useful things someone can type, and
	// hyphens must survive.
	if got := normaliseQuery("ballon-dor-winner-2026"); got != "ballon-dor-winner-2026" {
		t.Errorf("slug was mangled: %q", got)
	}
}

func TestNormaliseQueryFallsBackRatherThanReturningNothing(t *testing.T) {
	// All stop words. An empty search is worse than a poor one.
	if got := normaliseQuery("what is the"); got == "" {
		t.Error("normalising must not produce an empty query")
	}
}

type stubSearch struct {
	results  *domain.SearchResults
	err      error
	gotQuery string
	gotLimit int
}

func (s *stubSearch) SearchEvents(
	ctx context.Context, query string, limit int,
) (*domain.SearchResults, error) {
	s.gotQuery = query
	s.gotLimit = limit
	return s.results, s.err
}

func newSearchService(stub *stubSearch) Service {
	return NewPmService(&stubProvider{}, stub, slog.Default())
}

func TestSearchMarketsRejectsAnEmptyQuery(t *testing.T) {
	svc := newSearchService(&stubSearch{})

	var invalid shared.ErrInvalidInput
	if _, err := svc.SearchMarkets(context.Background(), SearchMarketsRequest{}); !errors.As(err, &invalid) {
		t.Fatalf("expected ErrInvalidInput for an empty query, got %v", err)
	}
	if _, err := svc.SearchMarkets(context.Background(), SearchMarketsRequest{Query: "   "}); !errors.As(err, &invalid) {
		t.Fatalf("expected ErrInvalidInput for a blank query, got %v", err)
	}
}

func TestSearchMarketsSendsNormalisedTerms(t *testing.T) {
	stub := &stubSearch{results: &domain.SearchResults{}}
	svc := newSearchService(stub)

	_, err := svc.SearchMarkets(
		context.Background(), SearchMarketsRequest{Query: "who will win the ballon dor?"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.gotQuery != "win ballon dor" {
		t.Errorf("provider received %q, want %q", stub.gotQuery, "win ballon dor")
	}
}

func TestSearchMarketsCapsTheLimit(t *testing.T) {
	stub := &stubSearch{results: &domain.SearchResults{}}
	svc := newSearchService(stub)

	if _, err := svc.SearchMarkets(
		context.Background(), SearchMarketsRequest{Query: "x", Limit: 500},
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.gotLimit != maxSearchLimit {
		t.Errorf("limit %d was not capped to %d", stub.gotLimit, maxSearchLimit)
	}

	if _, err := svc.SearchMarkets(
		context.Background(), SearchMarketsRequest{Query: "x"},
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.gotLimit != defaultSearchLimit {
		t.Errorf("limit %d is not the default %d", stub.gotLimit, defaultSearchLimit)
	}
}

func TestSearchMarketsExcludesSettledEvents(t *testing.T) {
	// A settled market's price can never move again, so analysing one is
	// pointless. world-cup-winner closed on 2026-07-20 and must not be
	// offered as a candidate.
	stub := &stubSearch{results: &domain.SearchResults{
		Events: []domain.SearchedEvent{
			{Slug: "live-event", Title: "Live", Active: true},
			{Slug: "settled-event", Title: "Settled", Active: true, Closed: true},
			{Slug: "archived-event", Title: "Archived", Active: true, Archived: true},
			{Slug: "inactive-event", Title: "Inactive", Active: false},
		},
	}}
	svc := newSearchService(stub)

	got, err := svc.SearchMarkets(context.Background(), SearchMarketsRequest{Query: "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got.Matches) != 1 || got.Matches[0].Slug != "live-event" {
		t.Errorf("settled events leaked into results: %+v", got.Matches)
	}
}

func TestSearchMarketsReturnsAnEmptyListNotNil(t *testing.T) {
	// A nil slice marshals to JSON null, which reads as "something went
	// wrong" rather than "nothing matched".
	stub := &stubSearch{results: &domain.SearchResults{}}
	svc := newSearchService(stub)

	got, err := svc.SearchMarkets(context.Background(), SearchMarketsRequest{Query: "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Matches == nil {
		t.Error("matches must be an empty list, not nil")
	}
}

func TestSearchMarketsPropagatesProviderErrors(t *testing.T) {
	svc := newSearchService(&stubSearch{err: errors.New("gamma down")})

	if _, err := svc.SearchMarkets(
		context.Background(), SearchMarketsRequest{Query: "x"},
	); err == nil {
		t.Error("expected the provider error to propagate")
	}
}
