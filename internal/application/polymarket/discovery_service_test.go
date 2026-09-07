package polymarket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	domain "github.com/TheGh0xt/Sagittarius/internal/domain/polymarket"
	"github.com/TheGh0xt/Sagittarius/internal/domain/shared"
)

// Events are built by decoding JSON rather than with struct literals: Event's
// Markets field is a large anonymous struct that cannot practically be written
// as a literal, and decoding exercises the real Gamma path — where every
// numeric field must tolerate arriving fractional.
func eventJSON(t *testing.T, slug string, endDate time.Time, change24h float64) domain.Event {
	t.Helper()

	raw := fmt.Sprintf(`{
		"id": "1",
		"slug": %q,
		"title": %q,
		"endDate": %q,
		"volume24hr": 1234.5,
		"markets": [{
			"id": "m1",
			"question": "will it happen?",
			"outcomePrices": "[\"0.42\", \"0.58\"]",
			"lastTradePrice": 0.42,
			"oneDayPriceChange": %v,
			"oneHourPriceChange": 0.01
		}]
	}`, slug, slug, endDate.Format(time.RFC3339), change24h)

	var ev domain.Event
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		t.Fatalf("fixture failed to decode: %v", err)
	}
	return ev
}

type stubDiscovery struct {
	byTag map[string][]domain.Event
	err   error
	calls []string
}

func (s *stubDiscovery) FetchEventsByTag(
	ctx context.Context, tagSlug string, limit int,
) ([]domain.Event, error) {
	s.calls = append(s.calls, tagSlug)
	if s.err != nil {
		return nil, s.err
	}
	return s.byTag[tagSlug], nil
}

func newDiscoveryService(d *stubDiscovery) Service {
	return NewPmServiceWithDiscovery(&stubProvider{}, &stubProvider{}, d, slog.Default())
}

// A category outside the UI_PRD §6.3 taxonomy must be rejected rather than
// silently querying a Gamma tag that does not exist — which returns zero
// events and reads as "nothing is moving".
func TestGetMovingMarketsRejectsUnknownCategory(t *testing.T) {
	svc := newDiscoveryService(&stubDiscovery{})

	var invalid shared.ErrInvalidInput
	_, err := svc.GetMovingMarkets(context.Background(), GetMovingMarketsRequest{
		Categories: []string{"Politics", "Underwater Basket Weaving"},
	})
	if !errors.As(err, &invalid) {
		t.Fatalf("expected ErrInvalidInput for unknown category, got %v", err)
	}
}

// The whole point of the survival filter: a market resolving inside the
// evaluation window settles to 0 or 1 and scores CONFIRMED trivially.
func TestGetMovingMarketsDropsMarketsResolvingTooSoon(t *testing.T) {
	now := time.Now().UTC()
	stub := &stubDiscovery{byTag: map[string][]domain.Event{
		"politics": {
			eventJSON(t, "resolves-tomorrow", now.Add(24*time.Hour), 0.40),
			eventJSON(t, "resolves-in-a-month", now.Add(30*24*time.Hour), 0.05),
		},
	}}

	got, err := newDiscoveryService(stub).GetMovingMarkets(
		context.Background(),
		GetMovingMarketsRequest{Categories: []string{"Politics"}},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got.Markets) != 1 {
		t.Fatalf("got %d markets, want 1: %+v", len(got.Markets), got.Markets)
	}
	if got.Markets[0].Slug != "resolves-in-a-month" {
		t.Errorf("kept %q; the market resolving tomorrow should have been dropped",
			got.Markets[0].Slug)
	}
}

// Round-robin across categories, so one busy category cannot monopolise the
// corpus and skew the accuracy record toward a single domain.
func TestGetMovingMarketsRoundRobinsAcrossCategories(t *testing.T) {
	now := time.Now().UTC()
	far := now.Add(60 * 24 * time.Hour)

	stub := &stubDiscovery{byTag: map[string][]domain.Event{
		// Politics has the three largest moves; without round-robin it would
		// take every slot.
		"politics": {
			eventJSON(t, "pol-a", far, 0.90),
			eventJSON(t, "pol-b", far, 0.80),
			eventJSON(t, "pol-c", far, 0.70),
		},
		"crypto": {eventJSON(t, "cry-a", far, 0.10)},
	}}

	got, err := newDiscoveryService(stub).GetMovingMarkets(
		context.Background(),
		GetMovingMarketsRequest{
			Categories: []string{"Politics", "Crypto"},
			Limit:      2,
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got.Markets) != 2 {
		t.Fatalf("got %d markets, want 2", len(got.Markets))
	}

	seen := map[string]bool{}
	for _, m := range got.Markets {
		seen[m.Category] = true
	}
	if !seen["Politics"] || !seen["Crypto"] {
		t.Errorf("expected one market from each category, got %+v", got.Markets)
	}
}

// No categories named means the full taxonomy — what the generator does while
// no users have onboarded.
func TestGetMovingMarketsDefaultsToTheWholeTaxonomy(t *testing.T) {
	stub := &stubDiscovery{byTag: map[string][]domain.Event{}}

	if _, err := newDiscoveryService(stub).GetMovingMarkets(
		context.Background(), GetMovingMarketsRequest{},
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Business & Earnings maps to two tags, so 13 categories query 14 tags.
	if len(stub.calls) != 14 {
		t.Errorf("queried %d tags, want 14: %v", len(stub.calls), stub.calls)
	}
}

// One category failing upstream must not lose the others. A partial feed is
// useful; an error page because Climate was down is not.
func TestGetMovingMarketsSurvivesOneCategoryFailing(t *testing.T) {
	now := time.Now().UTC()
	stub := &failingTagDiscovery{
		fail: "crypto",
		byTag: map[string][]domain.Event{
			"politics": {eventJSON(t, "pol-a", now.Add(60*24*time.Hour), 0.30)},
		},
	}

	got, err := NewPmServiceWithDiscovery(
		&stubProvider{}, &stubProvider{}, stub, slog.Default(),
	).GetMovingMarkets(context.Background(), GetMovingMarketsRequest{
		Categories: []string{"Politics", "Crypto"},
	})
	if err != nil {
		t.Fatalf("one failing category took down the whole call: %v", err)
	}
	if len(got.Markets) != 1 || got.Markets[0].Slug != "pol-a" {
		t.Errorf("lost the healthy category's markets: %+v", got.Markets)
	}
}

type failingTagDiscovery struct {
	byTag map[string][]domain.Event
	fail  string
}

func (f *failingTagDiscovery) FetchEventsByTag(
	ctx context.Context, tagSlug string, limit int,
) ([]domain.Event, error) {
	if tagSlug == f.fail {
		return nil, errors.New("upstream exploded")
	}
	return f.byTag[tagSlug], nil
}

// multiMarketEventJSON builds an event whose sub-markets each carry their own
// probability and 24h move.
func multiMarketEventJSON(t *testing.T, slug string, endDate time.Time, markets [][2]float64) domain.Event {
	t.Helper()

	parts := make([]string, 0, len(markets))
	for i, m := range markets {
		parts = append(parts, fmt.Sprintf(
			`{"id":"m%d","question":"q%d","lastTradePrice":%v,"oneDayPriceChange":%v,"active":true,"closed":false}`,
			i, i, m[0], m[1],
		))
	}

	raw := fmt.Sprintf(`{"id":"1","slug":%q,"title":%q,"endDate":%q,"volume24hr":1000,"markets":[%s]}`,
		slug, slug, endDate.Format(time.RFC3339), joinStrings(parts))

	var ev domain.Event
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		t.Fatalf("fixture failed to decode: %v", err)
	}
	return ev
}

func joinStrings(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ","
		}
		out += p
	}
	return out
}

// An event stays open while its individual sub-markets resolve. "Who will
// Trump endorse" ran to 2026-11-04 while one candidate's market settled to
// 0.999 — a +0.70 "move" that was a settlement, not a price movement.
//
// Picking the largest absolute move without checking whether that market can
// still move surfaces exactly those, and they are worse than useless: an
// already-settled market cannot move again, so an explanation of "why it
// moved" is explaining a resolution, and the T+48h check scores it CONFIRMED
// for free.
func TestGetMovingMarketsIgnoresSettledSubMarkets(t *testing.T) {
	far := time.Now().UTC().Add(60 * 24 * time.Hour)

	stub := &stubDiscovery{byTag: map[string][]domain.Event{
		"politics": {
			multiMarketEventJSON(t, "who-will-x-endorse", far, [][2]float64{
				{0.999, 0.702}, // settled YES — biggest move, must be ignored
				{0.001, -0.150}, // settled NO — must also be ignored
				{0.340, 0.080},  // the real, still-tradeable move
			}),
		},
	}}

	got, err := newDiscoveryService(stub).GetMovingMarkets(
		context.Background(),
		GetMovingMarketsRequest{Categories: []string{"Politics"}},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got.Markets) != 1 {
		t.Fatalf("got %d markets, want 1: %+v", len(got.Markets), got.Markets)
	}
	if got.Markets[0].Change24h != 0.080 {
		t.Errorf("lead move = %v (p=%v), want the tradeable 0.080 move; "+
			"a settled sub-market was chosen",
			got.Markets[0].Change24h, got.Markets[0].Probability)
	}
}

// If every sub-market has settled, the event has nothing left to explain and
// must drop out entirely rather than contributing a settled market.
func TestGetMovingMarketsDropsFullySettledEvents(t *testing.T) {
	far := time.Now().UTC().Add(60 * 24 * time.Hour)

	stub := &stubDiscovery{byTag: map[string][]domain.Event{
		"politics": {
			multiMarketEventJSON(t, "all-done", far, [][2]float64{
				{0.999, 0.50},
				{0.002, -0.40},
			}),
		},
	}}

	got, err := newDiscoveryService(stub).GetMovingMarkets(
		context.Background(),
		GetMovingMarketsRequest{Categories: []string{"Politics"}},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Markets) != 0 {
		t.Errorf("a fully settled event survived: %+v", got.Markets)
	}
}
