package polymarket

import (
	"encoding/json"
	"testing"
	"time"
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

// Gamma does not send every timestamp as RFC 3339. Some fields arrive in a
// Postgres-style layout — a space instead of the "T", and a two-digit offset
// like "+00" instead of "+00:00":
//
//	"umaEndDate": "2025-07-01 22:05:08.339341+00"
//
// Go's time.Time only unmarshals RFC 3339, and a single unparseable field
// fails the *entire* event array. Observed live on 2026-08-20, where it took
// out Politics, Crypto and Geopolitics completely while AI happened to be
// clean — three of four categories returning nothing, from one timestamp.
//
// This is the same failure mode as the fractional-number gotcha, and the same
// rule applies: one bad field must not cost the whole decode.
func TestEventDecodesPostgresStyleTimestamps(t *testing.T) {
	raw := []byte(`{
		"id": "1",
		"slug": "us-recession-by-end-of-2026",
		"endDate": "2026-12-31T12:00:00Z",
		"markets": [{
			"id": "m1",
			"question": "will it happen?",
			"umaEndDate": "2025-07-01 22:05:08.339341+00"
		}]
	}`)

	var ev Event
	if err := json.Unmarshal(raw, &ev); err != nil {
		t.Fatalf("event failed to decode: %v", err)
	}

	if ev.Slug != "us-recession-by-end-of-2026" {
		t.Errorf("slug = %q", ev.Slug)
	}
	if len(ev.Markets) != 1 {
		t.Fatalf("got %d markets, want 1", len(ev.Markets))
	}

	got := ev.Markets[0].UmaEndDate.UTC()
	want := time.Date(2025, 7, 1, 22, 5, 8, 339341000, time.UTC)
	if !got.Equal(want) {
		t.Errorf("umaEndDate = %v, want %v", got, want)
	}
}

// The ordinary RFC 3339 form must keep working — most timestamps use it.
func TestEventStillDecodesRFC3339Timestamps(t *testing.T) {
	raw := []byte(`{
		"id": "1",
		"markets": [{"id":"m1","umaEndDate":"2026-10-31T00:00:00Z"}]
	}`)

	var ev Event
	if err := json.Unmarshal(raw, &ev); err != nil {
		t.Fatalf("event failed to decode: %v", err)
	}
	if ev.Markets[0].UmaEndDate.UTC().Year() != 2026 {
		t.Errorf("umaEndDate = %v", ev.Markets[0].UmaEndDate)
	}
}

// An absent or empty timestamp is normal and must decode to the zero value
// rather than an error.
func TestEventDecodesEmptyTimestamp(t *testing.T) {
	raw := []byte(`{"id":"1","markets":[{"id":"m1","umaEndDate":""}]}`)

	var ev Event
	if err := json.Unmarshal(raw, &ev); err != nil {
		t.Fatalf("empty timestamp failed to decode: %v", err)
	}
	if !ev.Markets[0].UmaEndDate.IsZero() {
		t.Errorf("empty timestamp = %v, want zero", ev.Markets[0].UmaEndDate)
	}
}
