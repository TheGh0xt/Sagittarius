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
		{Wallet: "0xa", Side: "BUY", Price: 0.50, Size: 20000, Timestamp: 1751600000},  // $10k — below
		{Wallet: "0xb", Side: "BUY", Price: 0.50, Size: 50000, Timestamp: 1751600100},  // $25k — at threshold
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
		name         string
		trades       []polymarket.Trade
		baseline     float64
		wantVelocity float64
		wantSpike    bool
	}{
		{
			name:         "exactly +200pct is not a spike (boundary is strict)",
			trades:       []polymarket.Trade{{Price: 0.5, Size: 60000}}, // $30k recent
			baseline:     10000,
			wantVelocity: 200,
			wantSpike:    false,
		},
		{
			name:         "+300pct is a spike",
			trades:       []polymarket.Trade{{Price: 0.5, Size: 80000}}, // $40k recent
			baseline:     10000,
			wantVelocity: 300,
			wantSpike:    true,
		},
		{
			name:         "zero baseline yields zero velocity and no spike",
			trades:       []polymarket.Trade{{Price: 0.5, Size: 80000}},
			baseline:     0,
			wantVelocity: 0,
			wantSpike:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeVolumeSignal(tt.trades, tt.baseline)
			if !almostEqual(got.VelocityChangePct, tt.wantVelocity) {
				t.Errorf("velocity: want %v, got %v", tt.wantVelocity, got.VelocityChangePct)
			}
			if got.IsSpike != tt.wantSpike {
				t.Errorf("spike: want %v, got %v", tt.wantSpike, got.IsSpike)
			}
		})
	}
}
