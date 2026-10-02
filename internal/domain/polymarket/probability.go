package polymarket

import "encoding/json"

// YesProbability resolves a market's current YES price from a Gamma market.
//
// Gamma offers two price fields and they are not interchangeable:
//
//   - outcomePrices is the current mark, a JSON-encoded string array whose
//     first element is the YES leg: `["0.315","0.685"]`.
//   - lastTradePrice is the last print. On a thin book it is stale, on a
//     resolved or one-sided market it can sit on the opposite leg from the
//     one being quoted, and Gamma sends it as JSON `null` outright for about
//     40% of live markets.
//
// Sagittarius read lastTradePrice into a plain float64, so `null` decoded as
// the zero value and a market trading at 0.315 — or at 1.0 — was reported as
// probability zero. Measured over 1859 live markets on 2026-09-28: 755 null,
// and of the 1104 that did carry a value, 131 disagreed with outcomePrices by
// more than 0.02 and 26 by more than 0.50. Only 37% were right.
//
// So outcomePrices wins, and lastTradePrice is a fallback rather than the
// source. The boolean is false only when neither field yields a usable price
// — which did not occur once in that sample, but "nobody looked" must stay
// distinguishable from "zero". That is the same reasoning the signal DTO
// already applies to WhaleCount.
func YesProbability(outcomePrices string, lastTradePrice *float64) (float64, bool) {
	if outcomePrices != "" {
		var prices []string
		if err := json.Unmarshal([]byte(outcomePrices), &prices); err == nil && len(prices) > 0 {
			var yes float64
			// The elements are quoted decimals, so they need a second decode
			// rather than a cast — and Gamma has been seen to send "" for an
			// unpriced leg, which must fall through rather than become 0.
			if err := json.Unmarshal([]byte(prices[0]), &yes); err == nil {
				return yes, true
			}
		}
	}
	if lastTradePrice != nil {
		return *lastTradePrice, true
	}
	return 0, false
}
