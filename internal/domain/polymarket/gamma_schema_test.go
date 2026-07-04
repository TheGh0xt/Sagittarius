package polymarket

import (
	"encoding/json"
	"testing"
)

// Gamma returns fractional values for reward and AMM fields that were once
// assumed integral (live regression: rewardsDailyRate: 0.001 broke decoding).
func TestEventDecodesFractionalNumericFields(t *testing.T) {
	raw := `{
		"id": "30615",
		"title": "World Cup Winner",
		"markets": [{
			"question": "Will Spain win?",
			"conditionId": "0xcond",
			"orderMinSize": 5,
			"makerBaseFee": 0.5,
			"takerBaseFee": 0.25,
			"rewardsMinSize": 50.5,
			"volume24hrAmm": 123.45,
			"volumeAmm": 6789.01,
			"liquidityAmm": 42.5,
			"clobRewards": [{
				"id": "1",
				"rewardsAmount": 10.5,
				"rewardsDailyRate": 0.001
			}]
		}]
	}`

	var ev Event
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		t.Fatalf("failed to decode event with fractional fields: %v", err)
	}
	m := ev.Markets[0]
	if m.ClobRewards[0].RewardsDailyRate != 0.001 {
		t.Errorf("rewardsDailyRate: want 0.001, got %v", m.ClobRewards[0].RewardsDailyRate)
	}
	if m.Volume24HrAmm != 123.45 || m.MakerBaseFee != 0.5 {
		t.Errorf("fractional fields not preserved: %+v", m)
	}
}
