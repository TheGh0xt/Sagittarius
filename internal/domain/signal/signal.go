// Package signal contains the pure, deterministic math of the PMIE Signal
// Engine (Layer 2). No I/O happens here — every function is a pure
// transformation over domain market data, unit-testable without mocks.
package signal

import (
	"math"
	"strconv"
	"time"

	"github.com/TheGh0xt/Sagittarius/internal/domain/polymarket"
)

// WhaleEvent is a single trade whose USD notional meets the whale threshold.
type WhaleEvent struct {
	Wallet    string    `json:"wallet"`
	Side      string    `json:"side"`
	Price     float64   `json:"price"`
	Size      float64   `json:"size"`
	ValueUSD  float64   `json:"value_usd"`
	Timestamp time.Time `json:"timestamp"`
}

// Baseline sources, reported so the caller can tell a full trailing-week
// average from a degraded fallback.
const (
	BaselineTrailing7d  = "trailing_7d"
	BaselineLifetimeAvg = "lifetime_avg"
)

// volumeWindowHours is the span of the recent-volume figure. Both sides of the
// comparison are per-day rates, which is the whole point: the previous
// implementation compared a sum of the last 100 trades -- covering an hour on a
// busy market and months on a quiet one -- against a per-day baseline.
const volumeWindowHours = 24

// VolumeInput is the per-market volume data the signal is computed from.
//
// All of it comes from Gamma, which reports windowed volume directly. Deriving
// these from trades would cap the numerator at whatever the trade fetch limit
// returns, truncating exactly the busy markets a spike detector exists to find.
type VolumeInput struct {
	Volume24Hr float64 // recent: the last 24 hours
	Volume1Wk  float64 // trailing-week total, for the baseline
	Lifetime   float64 // fallback numerator
	AgeDays    float64 // real market age, supplied by the caller
}

// VolumeSignal compares the last 24 hours against a normal day and flags spikes.
type VolumeSignal struct {
	// Available is false when no baseline could be established. The rest of
	// the fields are then zero and mean nothing -- deliberately, rather than
	// carrying an invented constant that reads as a measurement.
	Available bool `json:"available"`

	RecentVolumeUSD   float64 `json:"recent_volume_usd"`
	BaselineVolumeUSD float64 `json:"baseline_volume_usd"`
	BaselineSource    string  `json:"baseline_source"`
	WindowHours       int     `json:"window_hours"`
	VelocityChangePct float64 `json:"velocity_change_pct"`
	IsSpike           bool    `json:"is_spike"` // velocity > +200%
}

// OrderbookSkew captures the imbalance between bid and ask dollar-weighted depth.
type OrderbookSkew struct {
	BidVolumeUSD float64 `json:"bid_volume_usd"`
	AskVolumeUSD float64 `json:"ask_volume_usd"`
	Skew         float64 `json:"skew"`   // (bid-ask)/(bid+ask) in [-1,1]
	Spread       float64 `json:"spread"` // |bestAsk - bestBid|
}

// ComputeWhaleEvents filters trades whose notional (price × size) >= thresholdUSD.
func ComputeWhaleEvents(trades []polymarket.Trade, thresholdUSD float64) []WhaleEvent {
	var events []WhaleEvent
	for _, t := range trades {
		val := t.Price * t.Size
		if val >= thresholdUSD {
			events = append(events, WhaleEvent{
				Wallet:    t.Wallet,
				Side:      t.Side,
				Price:     t.Price,
				Size:      t.Size,
				ValueUSD:  val,
				Timestamp: time.Unix(t.Timestamp, 0).UTC(),
			})
		}
	}
	return events
}

// ComputeOrderbookSkew calculates bid/ask dollar-weighted imbalance and spread.
func ComputeOrderbookSkew(ob *polymarket.Orderbook) OrderbookSkew {
	var bidVol, askVol float64

	for _, bid := range ob.Bids {
		p, _ := strconv.ParseFloat(bid.Price, 64)
		s, _ := strconv.ParseFloat(bid.Size, 64)
		bidVol += p * s
	}
	for _, ask := range ob.Asks {
		p, _ := strconv.ParseFloat(ask.Price, 64)
		s, _ := strconv.ParseFloat(ask.Size, 64)
		askVol += p * s
	}

	skew := 0.0
	if total := bidVol + askVol; total > 0 {
		skew = (bidVol - askVol) / total
	}

	spread := 0.0
	if len(ob.Asks) > 0 && len(ob.Bids) > 0 {
		bestAsk, _ := strconv.ParseFloat(ob.Asks[0].Price, 64)
		bestBid, _ := strconv.ParseFloat(ob.Bids[0].Price, 64)
		spread = math.Abs(bestAsk - bestBid)
	}

	return OrderbookSkew{
		BidVolumeUSD: bidVol,
		AskVolumeUSD: askVol,
		Skew:         skew,
		Spread:       spread,
	}
}

// ComputeVolumeSignal compares the last 24 hours of volume against a normal
// day for that market, flagging a spike above +200% (three times normal).
//
// The baseline is the market's own trailing-week daily average where one
// exists. Anything wider is worse: a lifetime average silently assumes the
// market's whole history resembles this week, and dividing lifetime volume by
// a guessed 30-day life -- what this did before -- overstated a normal day by a
// median of 12x on live data. Because the baseline is the denominator, too
// large a value shrinks the velocity, so a genuine spike reads as a drought and
// the detector misses precisely what it exists to catch.
func ComputeVolumeSignal(in VolumeInput) VolumeSignal {
	baseline, source := resolveBaseline(in)
	if baseline <= 0 {
		return VolumeSignal{Available: false}
	}

	velocityChange := ((in.Volume24Hr - baseline) / baseline) * 100

	return VolumeSignal{
		Available:         true,
		RecentVolumeUSD:   in.Volume24Hr,
		BaselineVolumeUSD: baseline,
		BaselineSource:    source,
		WindowHours:       volumeWindowHours,
		VelocityChangePct: velocityChange,
		IsSpike:           velocityChange > 200.0,
	}
}

// resolveBaseline picks a per-day baseline. A zero baseline means none could
// be formed.
func resolveBaseline(in VolumeInput) (float64, string) {
	// A market younger than a week has not lived a full trailing week, so
	// volume1wk holds only the days it existed. Dividing that by 7 averages in
	// days before it was listed, understating a normal day and over-firing the
	// spike flag -- the same error as the old lifetime/30 heuristic, in the
	// opposite direction. Its real age gives the honest rate.
	if in.AgeDays > 0 && in.AgeDays < 7 && in.Lifetime > 0 {
		return in.Lifetime / in.AgeDays, BaselineLifetimeAvg
	}
	if in.Volume1Wk > 0 {
		return in.Volume1Wk / 7.0, BaselineTrailing7d
	}
	if in.Lifetime > 0 && in.AgeDays > 0 {
		return in.Lifetime / in.AgeDays, BaselineLifetimeAvg
	}
	return 0, ""
}
