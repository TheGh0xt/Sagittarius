package polymarket

import "testing"

func f(v float64) *float64 { return &v }

// Every case here is a shape observed in a live Gamma response on 2026-09-28,
// not one invented to match the struct. That distinction is the point: the
// existing fixtures in this package set lastTradePrice to a number that
// happens to agree with outcomePrices, which is the one shape where a correct
// and an incorrect implementation are indistinguishable.
func TestYesProbability(t *testing.T) {
	cases := []struct {
		name           string
		outcomePrices  string
		lastTradePrice *float64
		want           float64
		wantOK         bool
	}{
		{
			// 755 of 1859 live markets. Decoded into a plain float64 this
			// became 0.0 — a market at 31.5% reported as impossible.
			name:          "null lastTradePrice still has a real outcomePrice",
			outcomePrices: `["0.315","0.685"]`,
			want:          0.315,
			wantOK:        true,
		},
		{
			// The inversion case: the book says certainty, the last print
			// says coin-flip. 26 live markets were off by 0.50 or more.
			name:           "outcomePrices beats a contradicting lastTradePrice",
			outcomePrices:  `["1","0"]`,
			lastTradePrice: f(0.4),
			want:           1,
			wantOK:         true,
		},
		{
			name:           "and the other direction",
			outcomePrices:  `["0","1"]`,
			lastTradePrice: f(0.55),
			want:           0,
			wantOK:         true,
		},
		{
			name:           "they usually agree, and that must keep working",
			outcomePrices:  `["0.62","0.38"]`,
			lastTradePrice: f(0.63),
			want:           0.62,
			wantOK:         true,
		},
		{
			// A genuine 0 must survive: it is a real statement about a real
			// market, not a missing value.
			name:          "a market really priced at zero is not 'no data'",
			outcomePrices: `["0","1"]`,
			want:          0,
			wantOK:        true,
		},
		{
			name:           "falls back to lastTradePrice when outcomePrices is absent",
			outcomePrices:  "",
			lastTradePrice: f(0.42),
			want:           0.42,
			wantOK:         true,
		},
		{
			name:           "falls back when outcomePrices is malformed rather than guessing",
			outcomePrices:  `not json`,
			lastTradePrice: f(0.42),
			want:           0.42,
			wantOK:         true,
		},
		{
			name:           "an empty price string falls through instead of becoming zero",
			outcomePrices:  `["","0.5"]`,
			lastTradePrice: f(0.42),
			want:           0.42,
			wantOK:         true,
		},
		{
			name:          "no price anywhere is reported as such, never as zero",
			outcomePrices: "",
			want:          0,
			wantOK:        false,
		},
		{
			name:          "an empty array is not a price",
			outcomePrices: `[]`,
			want:          0,
			wantOK:        false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := YesProbability(tc.outcomePrices, tc.lastTradePrice)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if got != tc.want {
				t.Errorf("probability = %v, want %v", got, tc.want)
			}
		})
	}
}
