package polymarket

import "github.com/TheGh0xt/Sagittarius/internal/domain/polymarket"

type (
	EventSummary struct {
		Title        string  `json:"title"`
		Volume       float64 `json:"volume"`
		Volume24h    float64 `json:"volume_24h"`
		Liquidity    float64 `json:"liquidity"`
		OpenInterest float64 `json:"open_interest"`
	}

	MarketAnalysis struct {
		// ConditionID is the market's stable on-chain identifier, and the
		// value downstream systems must key on. Without it the reasoning
		// agent has nothing to cite, and it will invent an identifier from
		// whatever text it has — a stored report is then impossible to group
		// by market or evaluate against one.
		ConditionID string `json:"condition_id"`

		// Slug identifies the market within its event and is what a human
		// recognises in a Polymarket URL.
		Slug string `json:"slug"`

		Question string `json:"question"`

		Probability float64 `json:"probability"`

		Change1h  float64 `json:"change_1h"`
		Change24h float64 `json:"change_24h"`
		Change7d  float64 `json:"change_7d"`
		Change30d float64 `json:"change_30d"`

		Volume24h float64 `json:"volume_24h"`
		Liquidity float64 `json:"liquidity"`

		BestBid float64 `json:"best_bid"`
		BestAsk float64 `json:"best_ask"`
		Spread  float64 `json:"spread"`
	}
)

func BuildEventIntelligenceContext(event *polymarket.Event) *EventIntelligenceContext {
	ctx := &EventIntelligenceContext{
		Event: EventSummary{
			Title:        event.Title,
			Volume:       event.Volume,
			Volume24h:    event.Volume24Hr,
			Liquidity:    event.Liquidity,
			OpenInterest: event.OpenInterest,
		},
		Context: event.EventMetadata.ContextDescription,
	}

	for _, tag := range event.Tags {
		ctx.Tags = append(ctx.Tags, tag.Label)
	}

	for _, market := range event.Markets {
		ctx.Markets = append(ctx.Markets, MarketAnalysis{
			ConditionID: market.ConditionID,
			Slug:        market.Slug,

			Question: market.Question,

			// SAG-33. See internal/domain/polymarket/probability.go — the last
			// print is the wrong source, and it is null far more often than not.
			Probability: yesProbability(market.OutcomePrices, market.LastTradePrice),

			Change1h:  float64(market.OneHourPriceChange),
			Change24h: market.OneDayPriceChange,
			Change7d:  market.OneWeekPriceChange,
			Change30d: market.OneMonthPriceChange,

			Volume24h: market.Volume24Hr,
			Liquidity: market.LiquidityClob,

			BestBid: market.BestBid,
			BestAsk: market.BestAsk,
			Spread:  market.Spread,
		})
	}

	return ctx
}

// yesProbability is domain.YesProbability with the "no price anywhere" case
// flattened to 0, for DTO fields that cannot yet express absence.
//
// Safe only because that case is vanishingly rare — 0 of 1859 live markets
// sampled on 2026-09-28 — and because 0 is also what the consumer treats as
// "no usable market data". Making these fields nullable is the remaining half
// of SAG-33 and is a cross-repo contract change; see Cygnus#81.
func yesProbability(outcomePrices string, lastTradePrice *float64) float64 {
	p, _ := polymarket.YesProbability(outcomePrices, lastTradePrice)
	return p
}
