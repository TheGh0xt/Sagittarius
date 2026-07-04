package signal

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"testing"

	domain "github.com/TheGh0xt/Sagittarius/internal/domain/polymarket"
	"github.com/TheGh0xt/Sagittarius/internal/domain/shared"
)

// testEvent builds an Event via JSON because Event.Markets is an anonymous
// struct type that cannot be constructed field-by-field from another package.
func testEvent() *domain.Event {
	raw := `{
		"id": "1",
		"title": "Will BTC hit 150k?",
		"markets": [{
			"question": "Will BTC hit $150k by Dec 2026?",
			"conditionId": "0xcond",
			"clobTokenIds": "[\"tok1\",\"tok2\"]",
			"volumeNum": 300000,
			"lastTradePrice": 0.58
		}]
	}`
	var ev domain.Event
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		panic(err)
	}
	return &ev
}

type stubEventProvider struct {
	event *domain.Event
	err   error
}

func (s *stubEventProvider) FetchEventBySlug(ctx context.Context, slug string) (*domain.Event, error) {
	return s.event, s.err
}

func (s *stubEventProvider) FetchEventByID(ctx context.Context, id string) (*domain.Event, error) {
	return s.event, s.err
}

type stubMarketData struct {
	trades    []domain.Trade
	tradesErr error
	book      *domain.Orderbook
	bookErr   error
}

func (s *stubMarketData) FetchTrades(ctx context.Context, conditionID string, limit int) ([]domain.Trade, error) {
	return s.trades, s.tradesErr
}

func (s *stubMarketData) FetchOrderbook(ctx context.Context, tokenID string) (*domain.Orderbook, error) {
	return s.book, s.bookErr
}

var whaleTrades = []domain.Trade{
	{Wallet: "0xa", Side: "BUY", Price: 0.5, Size: 60000, Timestamp: 1751600000},   // $30k
	{Wallet: "0xb", Side: "SELL", Price: 0.5, Size: 20000, Timestamp: 1751600100},  // $10k — below threshold
	{Wallet: "0xc", Side: "SELL", Price: 0.5, Size: 100000, Timestamp: 1751600200}, // $50k
}

var testBook = &domain.Orderbook{
	AssetID: "tok1",
	Asks:    []domain.OrderbookLevel{{Price: "0.55", Size: "100"}, {Price: "0.56", Size: "200"}},
	Bids:    []domain.OrderbookLevel{{Price: "0.54", Size: "150"}, {Price: "0.53", Size: "300"}},
}

func TestDetectWhaleActivityValidation(t *testing.T) {
	svc := NewSignalService(&stubEventProvider{}, &stubMarketData{}, slog.Default())

	var invalid shared.ErrInvalidInput
	if _, err := svc.DetectWhaleActivity(context.Background(), WhaleActivityRequest{Slug: ""}); !errors.As(err, &invalid) {
		t.Fatalf("expected ErrInvalidInput for empty slug, got %v", err)
	}
}

func TestDetectWhaleActivity(t *testing.T) {
	svc := NewSignalService(
		&stubEventProvider{event: testEvent()},
		&stubMarketData{trades: whaleTrades, book: testBook},
		slog.Default(),
	)

	report, err := svc.DetectWhaleActivity(context.Background(), WhaleActivityRequest{Slug: "will-btc-hit-150k"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.EventTitle != "Will BTC hit 150k?" || report.USDThreshold != 25000 {
		t.Errorf("unexpected report header: %+v", report)
	}
	if len(report.Markets) != 1 {
		t.Fatalf("expected 1 market, got %d", len(report.Markets))
	}
	m := report.Markets[0]
	if m.ConditionID != "0xcond" {
		t.Errorf("unexpected condition id: %s", m.ConditionID)
	}
	if len(m.WhaleEvents) != 2 {
		t.Fatalf("expected 2 whale events, got %d", len(m.WhaleEvents))
	}
	if m.TotalValueUSD != 80000 {
		t.Errorf("total whale value: want 80000, got %v", m.TotalValueUSD)
	}
	// BUY $30k of $80k = 37.5% → rounds to 38
	if m.BuySellRatio != "38:62" {
		t.Errorf("buy/sell ratio: want 38:62, got %s", m.BuySellRatio)
	}
}

func TestDetectWhaleActivityNoWhales(t *testing.T) {
	svc := NewSignalService(
		&stubEventProvider{event: testEvent()},
		&stubMarketData{trades: nil, book: testBook},
		slog.Default(),
	)

	report, err := svc.DetectWhaleActivity(context.Background(), WhaleActivityRequest{Slug: "s"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := report.Markets[0].BuySellRatio; got != "0:0" {
		t.Errorf("buy/sell ratio with no whales: want 0:0, got %s", got)
	}
	if report.Markets[0].WhaleEvents == nil {
		t.Error("whale_events must serialize as [] (empty slice), not null")
	}
}

func TestDetectWhaleActivitySkipsFailingMarkets(t *testing.T) {
	svc := NewSignalService(
		&stubEventProvider{event: testEvent()},
		&stubMarketData{tradesErr: errors.New("upstream down")},
		slog.Default(),
	)

	report, err := svc.DetectWhaleActivity(context.Background(), WhaleActivityRequest{Slug: "s"})
	if err != nil {
		t.Fatalf("degraded report should not error: %v", err)
	}
	if len(report.Markets) != 0 {
		t.Errorf("expected failing market to be skipped, got %d markets", len(report.Markets))
	}
}

func TestBuildMarketSnapshot(t *testing.T) {
	svc := NewSignalService(
		&stubEventProvider{event: testEvent()},
		&stubMarketData{trades: whaleTrades, book: testBook},
		slog.Default(),
	)

	report, err := svc.BuildMarketSnapshot(context.Background(), "will-btc-hit-150k")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.Markets) != 1 {
		t.Fatalf("expected 1 market, got %d", len(report.Markets))
	}
	m := report.Markets[0]
	if m.Probability != 0.58 {
		t.Errorf("probability: want 0.58, got %v", m.Probability)
	}
	if math.Abs(m.SkewInfo.BidVolumeUSD-240) > 1e-9 || math.Abs(m.SkewInfo.AskVolumeUSD-167) > 1e-9 {
		t.Errorf("unexpected skew: %+v", m.SkewInfo)
	}
	if m.WhaleCount != 2 {
		t.Errorf("whale count: want 2, got %d", m.WhaleCount)
	}
	// recent $90k vs baseline $10k = +800% → spike
	if !m.VolumeAnalysis.IsSpike {
		t.Errorf("expected volume spike, got %+v", m.VolumeAnalysis)
	}
}

func TestBuildMarketSnapshotValidation(t *testing.T) {
	svc := NewSignalService(&stubEventProvider{}, &stubMarketData{}, slog.Default())

	var invalid shared.ErrInvalidInput
	if _, err := svc.BuildMarketSnapshot(context.Background(), ""); !errors.As(err, &invalid) {
		t.Fatalf("expected ErrInvalidInput for empty slug, got %v", err)
	}
}

func TestFirstClobTokenID(t *testing.T) {
	if got, err := firstClobTokenID(`["tok1","tok2"]`); err != nil || got != "tok1" {
		t.Errorf("want tok1, got %q err %v", got, err)
	}
	if _, err := firstClobTokenID(`[]`); err == nil {
		t.Error("expected error for empty token list")
	}
	if _, err := firstClobTokenID(`not-json`); err == nil {
		t.Error("expected error for malformed token list")
	}
}
