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

// VolumeSignal compares recent trade volume against a baseline and flags spikes.
type VolumeSignal struct {
	RecentVolumeUSD   float64 `json:"recent_volume_usd"`
	BaselineVolumeUSD float64 `json:"baseline_volume_usd"`
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

// ComputeVolumeSignal compares recent trade volume against a baseline average
// and flags whether the velocity change constitutes a spike (> +200%).
func ComputeVolumeSignal(trades []polymarket.Trade, baselineVolumeUSD float64) VolumeSignal {
	recentVol := 0.0
	for _, t := range trades {
		recentVol += t.Price * t.Size
	}

	velocityChange := 0.0
	if baselineVolumeUSD > 0 {
		velocityChange = ((recentVol - baselineVolumeUSD) / baselineVolumeUSD) * 100
	}

	return VolumeSignal{
		RecentVolumeUSD:   recentVol,
		BaselineVolumeUSD: baselineVolumeUSD,
		VelocityChangePct: velocityChange,
		IsSpike:           velocityChange > 200.0,
	}
}
