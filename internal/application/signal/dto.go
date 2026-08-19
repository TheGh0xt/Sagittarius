package signal

import (
	"time"

	"github.com/TheGh0xt/Sagittarius/internal/domain/signal"
)

type (
	WhaleActivityRequest struct {
		Slug         string  `json:"slug" binding:"required" jsonschema:"Polymarket event slug (the path segment after /event/ in the URL)"`
		USDThreshold float64 `json:"usd_threshold,omitempty" jsonschema:"minimum USD notional to classify a trade as a whale event; defaults to 25000"`
		Limit        int     `json:"limit,omitempty" jsonschema:"max trades to scan per market; defaults to 100, capped at 500"`
	}

	MarketSnapshotRequest struct {
		Slug string `json:"slug" binding:"required" jsonschema:"Polymarket event slug (the path segment after /event/ in the URL)"`
	}

	MarketWhaleActivity struct {
		Question      string              `json:"question"`
		ConditionID   string              `json:"condition_id"`
		WhaleEvents   []signal.WhaleEvent `json:"whale_events"`
		TotalValueUSD float64             `json:"total_whale_value_usd"`
		BuySellRatio  string              `json:"buy_sell_ratio"` // by USD value, e.g. "87:13"
	}

	WhaleActivityReport struct {
		EventTitle   string                `json:"event_title"`
		USDThreshold float64               `json:"usd_threshold"`
		Markets      []MarketWhaleActivity `json:"markets"`
	}

	MarketSnapshot struct {
		Question       string               `json:"question"`
		ConditionID    string               `json:"condition_id"`
		Probability    float64              `json:"probability"`
		SkewInfo       signal.OrderbookSkew `json:"skew_info"`
		VolumeAnalysis signal.VolumeSignal  `json:"volume_analysis"`
		// Null when the trade fetch failed, which is not the same statement as
		// zero. Reporting 0 there would tell the reasoning agent this market
		// has no whale activity when what happened is that nobody looked.
		WhaleCount *int `json:"whale_count"`
	}

	MarketSnapshotReport struct {
		EventTitle string           `json:"event_title"`
		Timestamp  time.Time        `json:"timestamp"`
		Markets    []MarketSnapshot `json:"markets"`
	}
)
