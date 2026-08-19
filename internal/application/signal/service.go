package signal

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/TheGh0xt/Sagittarius/internal/domain/polymarket"
	"github.com/TheGh0xt/Sagittarius/internal/domain/shared"
	"github.com/TheGh0xt/Sagittarius/internal/domain/signal"
)

const (
	defaultUSDThreshold = 25000.0
	defaultTradeLimit   = 100
	maxTradeLimit       = 500
)

type Service interface {
	DetectWhaleActivity(ctx context.Context, req WhaleActivityRequest) (*WhaleActivityReport, error)
	BuildMarketSnapshot(ctx context.Context, slug string) (*MarketSnapshotReport, error)
}

type signalService struct {
	ep  polymarket.EventProvider
	mdp polymarket.MarketDataProvider
	slg *slog.Logger
}

func NewSignalService(
	ep polymarket.EventProvider,
	mdp polymarket.MarketDataProvider,
	slg *slog.Logger,
) Service {
	return &signalService{
		ep:  ep,
		mdp: mdp,
		slg: slg,
	}
}

func (s *signalService) DetectWhaleActivity(ctx context.Context, req WhaleActivityRequest) (*WhaleActivityReport, error) {
	if req.Slug == "" {
		return nil, shared.ErrInvalidInput{Field: "slug", Message: "event slug cannot be empty"}
	}
	threshold := req.USDThreshold
	if threshold <= 0 {
		threshold = defaultUSDThreshold
	}
	limit := req.Limit
	if limit <= 0 {
		limit = defaultTradeLimit
	}
	if limit > maxTradeLimit {
		limit = maxTradeLimit
	}

	event, err := s.ep.FetchEventBySlug(ctx, req.Slug)
	if err != nil {
		return nil, err
	}

	report := &WhaleActivityReport{
		EventTitle:   event.Title,
		USDThreshold: threshold,
		Markets:      []MarketWhaleActivity{},
	}

	for i := range event.Markets {
		market := &event.Markets[i]

		trades, err := s.mdp.FetchTrades(ctx, market.ConditionID, limit)
		if err != nil {
			// A degraded report beats a hard failure: skip this market.
			s.slg.Warn("skipping market: trades fetch failed",
				"condition_id", market.ConditionID, "err", err)
			continue
		}

		events := signal.ComputeWhaleEvents(trades, threshold)
		if events == nil {
			// Serialize as [] rather than null for tool consumers.
			events = []signal.WhaleEvent{}
		}

		var total, buyUSD float64
		for _, e := range events {
			total += e.ValueUSD
			if e.Side == "BUY" {
				buyUSD += e.ValueUSD
			}
		}

		report.Markets = append(report.Markets, MarketWhaleActivity{
			Question:      market.Question,
			ConditionID:   market.ConditionID,
			WhaleEvents:   events,
			TotalValueUSD: total,
			BuySellRatio:  buySellRatio(buyUSD, total),
		})
	}

	return report, nil
}

func (s *signalService) BuildMarketSnapshot(ctx context.Context, slug string) (*MarketSnapshotReport, error) {
	if slug == "" {
		return nil, shared.ErrInvalidInput{Field: "slug", Message: "event slug cannot be empty"}
	}

	event, err := s.ep.FetchEventBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	// Read once so every market in a report is aged against the same instant.
	now := time.Now().UTC()

	report := &MarketSnapshotReport{
		EventTitle: event.Title,
		Timestamp:  now,
		Markets:    []MarketSnapshot{},
	}

	for i := range event.Markets {
		market := &event.Markets[i]

		// Only the whale count comes from trades now: volume is read from
		// Gamma's windowed fields. So a trades outage costs one field rather
		// than the whole market, which is what dropping it here used to do.
		var whaleCount *int
		if trades, err := s.mdp.FetchTrades(ctx, market.ConditionID, defaultTradeLimit); err != nil {
			s.slg.Warn("trades fetch failed; whale count omitted",
				"condition_id", market.ConditionID, "err", err)
		} else {
			count := len(signal.ComputeWhaleEvents(trades, defaultUSDThreshold))
			whaleCount = &count
		}

		var skew signal.OrderbookSkew
		if tokenID, err := firstClobTokenID(market.ClobTokenIds); err != nil {
			s.slg.Warn("no clob token for market; skew omitted",
				"condition_id", market.ConditionID, "err", err)
		} else if book, err := s.mdp.FetchOrderbook(ctx, tokenID); err != nil {
			s.slg.Warn("orderbook fetch failed; skew omitted",
				"condition_id", market.ConditionID, "err", err)
		} else {
			skew = signal.ComputeOrderbookSkew(book)
		}

		report.Markets = append(report.Markets, MarketSnapshot{
			Question:    market.Question,
			ConditionID: market.ConditionID,
			Probability: market.LastTradePrice,
			SkewInfo:    skew,
			VolumeAnalysis: signal.ComputeVolumeSignal(signal.VolumeInput{
				Volume24Hr: market.Volume24Hr,
				Volume1Wk:  market.Volume1Wk,
				Lifetime:   market.VolumeNum,
				AgeDays:    marketAgeDays(market.StartDate, now),
			}),
			WhaleCount: whaleCount,
		})
	}

	return report, nil
}

// marketAgeDays is how long a market has been open, in fractional days.
//
// Returns 0 when the start date is missing or in the future, which the domain
// reads as "no age known" and refuses to build a baseline from -- rather than
// dividing by a guess, which is the mistake this replaces.
func marketAgeDays(startDate, now time.Time) float64 {
	if startDate.IsZero() || !startDate.Before(now) {
		return 0
	}
	return now.Sub(startDate).Hours() / 24.0
}

// buySellRatio renders whale USD value split as "BUY:SELL" percentages.
func buySellRatio(buyUSD, totalUSD float64) string {
	if totalUSD <= 0 {
		return "0:0"
	}
	buyPct := int(math.Round(buyUSD / totalUSD * 100))
	return fmt.Sprintf("%d:%d", buyPct, 100-buyPct)
}

// firstClobTokenID extracts the first token from Gamma's clobTokenIds field,
// which is a JSON-encoded string array (e.g. "[\"123\",\"456\"]").
func firstClobTokenID(raw string) (string, error) {
	var ids []string
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return "", fmt.Errorf("malformed clobTokenIds %q: %w", raw, err)
	}
	if len(ids) == 0 || ids[0] == "" {
		return "", fmt.Errorf("empty clobTokenIds")
	}
	return ids[0], nil
}
