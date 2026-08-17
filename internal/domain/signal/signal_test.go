package signal

import (
	"math"
	"testing"

	"github.com/TheGh0xt/Sagittarius/internal/domain/polymarket"
)

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestComputeWhaleEvents(t *testing.T) {
	trades := []polymarket.Trade{
		{Wallet: "0xa", Side: "BUY", Price: 0.50, Size: 20000, Timestamp: 1751600000},   // $10k — below
		{Wallet: "0xb", Side: "BUY", Price: 0.50, Size: 50000, Timestamp: 1751600100},   // $25k — at threshold
		{Wallet: "0xc", Side: "SELL", Price: 0.60, Size: 100000, Timestamp: 1751600200}, // $60k — above
	}

	events := ComputeWhaleEvents(trades, 25000)
	if len(events) != 2 {
		t.Fatalf("expected 2 whale events, got %d", len(events))
	}
	if events[0].Wallet != "0xb" || !almostEqual(events[0].ValueUSD, 25000) {
		t.Errorf("unexpected first event: %+v", events[0])
	}
	if events[1].Side != "SELL" || !almostEqual(events[1].ValueUSD, 60000) {
		t.Errorf("unexpected second event: %+v", events[1])
	}

	if got := ComputeWhaleEvents(nil, 25000); got != nil {
		t.Errorf("expected nil for no trades, got %+v", got)
	}
}

func TestComputeOrderbookSkew(t *testing.T) {
	// The prototype's documented example:
	// askVol = 0.55*100 + 0.56*200 = 167; bidVol = 0.54*150 + 0.53*300 = 240
	ob := &polymarket.Orderbook{
		Asks: []polymarket.OrderbookLevel{
			{Price: "0.55", Size: "100"},
			{Price: "0.56", Size: "200"},
		},
		Bids: []polymarket.OrderbookLevel{
			{Price: "0.54", Size: "150"},
			{Price: "0.53", Size: "300"},
		},
	}

	skew := ComputeOrderbookSkew(ob)
	if !almostEqual(skew.BidVolumeUSD, 240) {
		t.Errorf("bid volume: want 240, got %v", skew.BidVolumeUSD)
	}
	if !almostEqual(skew.AskVolumeUSD, 167) {
		t.Errorf("ask volume: want 167, got %v", skew.AskVolumeUSD)
	}
	wantSkew := (240.0 - 167.0) / (240.0 + 167.0)
	if !almostEqual(skew.Skew, wantSkew) {
		t.Errorf("skew: want %v, got %v", wantSkew, skew.Skew)
	}
	if math.Abs(skew.Spread-0.01) > 1e-9 {
		t.Errorf("spread: want 0.01, got %v", skew.Spread)
	}
}

func TestComputeOrderbookSkewEmptyBook(t *testing.T) {
	skew := ComputeOrderbookSkew(&polymarket.Orderbook{})
	if skew.BidVolumeUSD != 0 || skew.AskVolumeUSD != 0 || skew.Skew != 0 || skew.Spread != 0 {
		t.Errorf("expected zero skew for empty book, got %+v", skew)
	}
}

func TestComputeVolumeSignal(t *testing.T) {
	tests := []struct {
		name          string
		in            VolumeInput
		wantAvailable bool
		wantBaseline  float64
		wantSource    string
		wantVelocity  float64
		wantSpike     bool
	}{
		{
			// The trailing week is the preferred baseline: a full, authoritative
			// figure covering seven days, divided into a per-day rate that the
			// 24h numerator can be compared against directly.
			name:          "trailing week sets the baseline",
			in:            VolumeInput{Volume24Hr: 12000, Volume1Wk: 84000, Lifetime: 1500000, AgeDays: 330},
			wantAvailable: true,
			wantBaseline:  12000, // 84000 / 7
			wantSource:    BaselineTrailing7d,
			wantVelocity:  0,
			wantSpike:     false,
		},
		{
			name:          "exactly +200pct is not a spike (boundary is strict)",
			in:            VolumeInput{Volume24Hr: 30000, Volume1Wk: 70000},
			wantAvailable: true,
			wantBaseline:  10000,
			wantSource:    BaselineTrailing7d,
			wantVelocity:  200,
			wantSpike:     false,
		},
		{
			name:          "+300pct is a spike",
			in:            VolumeInput{Volume24Hr: 40000, Volume1Wk: 70000},
			wantAvailable: true,
			wantBaseline:  10000,
			wantSource:    BaselineTrailing7d,
			wantVelocity:  300,
			wantSpike:     true,
		},
		{
			// A market younger than a week has no trailing week to average, so
			// it falls back to its real lifetime rate -- real age, never a
			// hardcoded 30 days, which is what made the old heuristic overstate
			// a normal day by a median of 12x.
			name:          "empty week falls back to lifetime over real age",
			in:            VolumeInput{Volume24Hr: 9000, Volume1Wk: 0, Lifetime: 12000, AgeDays: 4},
			wantAvailable: true,
			wantBaseline:  3000, // 12000 / 4
			wantSource:    BaselineLifetimeAvg,
			wantVelocity:  200,
			wantSpike:     false,
		},
		{
			// Found in live verification: Polymarket lists sports markets
			// constantly, so young markets are common. volume1wk holds only the
			// days such a market existed, and dividing it by 7 averages in days
			// before it was listed -- understating a normal day and reporting a
			// spike that did not happen.
			name:          "market younger than a week uses its real age, not a 7-day divisor",
			in:            VolumeInput{Volume24Hr: 30000, Volume1Wk: 60000, Lifetime: 60000, AgeDays: 3},
			wantAvailable: true,
			wantBaseline:  20000, // 60000 / 3, not 60000 / 7
			wantSource:    BaselineLifetimeAvg,
			wantVelocity:  50,
			wantSpike:     false,
		},
		{
			// Reported absent rather than defaulted. A fabricated baseline
			// reaches the LLM as fact and gets cited as though it were measured.
			name:          "no volume anywhere is unavailable, not defaulted",
			in:            VolumeInput{},
			wantAvailable: false,
			wantBaseline:  0,
			wantSource:    "",
			wantVelocity:  0,
			wantSpike:     false,
		},
		{
			name:          "lifetime with unknown age cannot form a rate",
			in:            VolumeInput{Volume24Hr: 500, Lifetime: 12000, AgeDays: 0},
			wantAvailable: false,
			wantBaseline:  0,
			wantSource:    "",
			wantVelocity:  0,
			wantSpike:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeVolumeSignal(tt.in)

			if got.Available != tt.wantAvailable {
				t.Fatalf("available: want %v, got %v", tt.wantAvailable, got.Available)
			}
			if !almostEqual(got.BaselineVolumeUSD, tt.wantBaseline) {
				t.Errorf("baseline: want %v, got %v", tt.wantBaseline, got.BaselineVolumeUSD)
			}
			if got.BaselineSource != tt.wantSource {
				t.Errorf("source: want %q, got %q", tt.wantSource, got.BaselineSource)
			}
			if !almostEqual(got.VelocityChangePct, tt.wantVelocity) {
				t.Errorf("velocity: want %v, got %v", tt.wantVelocity, got.VelocityChangePct)
			}
			if got.IsSpike != tt.wantSpike {
				t.Errorf("spike: want %v, got %v", tt.wantSpike, got.IsSpike)
			}
		})
	}
}

// The window has to travel with the numbers. "recent_volume_usd: 12000" is
// meaningless to the reasoning agent unless it knows what "recent" spans, and
// an agent that assumes the wrong window draws the wrong conclusion from a
// correct figure.
func TestVolumeSignalStatesItsWindow(t *testing.T) {
	got := ComputeVolumeSignal(VolumeInput{Volume24Hr: 12000, Volume1Wk: 84000})
	if got.WindowHours != 24 {
		t.Errorf("window_hours: want 24, got %d", got.WindowHours)
	}
	if !almostEqual(got.RecentVolumeUSD, 12000) {
		t.Errorf("recent: want 12000, got %v", got.RecentVolumeUSD)
	}
}
