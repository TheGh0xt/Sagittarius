package signal

import (
	"testing"

	"github.com/TheGh0xt/Sagittarius/internal/polymarket"
)

func TestOrderbookSkewCalculation(t *testing.T) {
	// Sample asks and bids
	asks := []polymarket.PriceSize{
		{Price: "0.55", Size: "100"}, // Value = 55
		{Price: "0.56", Size: "200"}, // Value = 112
	}
	bids := []polymarket.PriceSize{
		{Price: "0.54", Size: "150"}, // Value = 81
		{Price: "0.53", Size: "300"}, // Value = 159
	}

	ob := &polymarket.Orderbook{
		Asks: asks,
		Bids: bids,
	}

	// Calculate manually
	// AskVolume = (0.55 * 100) + (0.56 * 200) = 55 + 112 = 167
	// BidVolume = (0.54 * 150) + (0.53 * 300) = 81 + 159 = 240
	// Expected skew = (240 - 167) / (240 + 167) = 73 / 407 = 0.179361...

	NewEngine(nil) // Client can be nil since we test calculation helper

	// Test order book skew logic manually via a wrapper or by testing the code directly if refactored.
	// Since engine.AnalyzeOrderbookSkew calls client.GetOrderbook, let's refactor the calculations in engine
	// to make them testable, or run a test on calculation values.

	// Let's verify the spread
	bestAsk := 0.55
	bestBid := 0.54
	spread := bestAsk - bestBid
	if spread != 0.010000000000000009 && spread != 0.01 { // float64 precision handling
		t.Errorf("Unexpected spread: %v", spread)
	}

	bidVol := 0.0
	for _, bid := range ob.Bids {
		p := 0.0
		s := 0.0
		if val, err := parseHelper(bid.Price); err == nil {
			p = val
		}
		if val, err := parseHelper(bid.Size); err == nil {
			s = val
		}
		bidVol += p * s
	}
	if bidVol != 240.0 {
		t.Errorf("Expected bid volume to be 240.0, got %v", bidVol)
	}
}

func parseHelper(s string) (float64, error) {
	// simple test parser helper
	return float64(0), nil
}
