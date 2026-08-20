package polymarket

import (
	"strings"
	"time"
)

// FlexibleTime is a timestamp that tolerates the several layouts Gamma
// actually emits.
//
// Gamma is not consistent. Most timestamps are RFC 3339, but some fields come
// back in a Postgres-style layout — a space instead of the "T", and a
// two-digit offset:
//
//	"umaEndDate": "2025-07-01 22:05:08.339341+00"
//
// encoding/json only accepts RFC 3339 for time.Time, and because a decode
// error aborts the whole document, one such field silently destroys an entire
// event array. Observed live on 2026-08-20: Politics, Crypto and Geopolitics
// all returned nothing while AI worked, because AI's events happened not to
// carry the malformed field.
//
// This is the timestamp twin of the fractional-number gotcha already recorded
// in this package, and it earns the same defence — parse leniently at the
// boundary so a single upstream inconsistency cannot cost the whole response.
type FlexibleTime struct {
	time.Time
}

// gammaTimeLayouts are tried in order. RFC 3339 first, because it is the
// common case.
var gammaTimeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02 15:04:05.999999-07",
	"2006-01-02 15:04:05.999999-07:00",
	"2006-01-02 15:04:05-07",
	"2006-01-02 15:04:05-07:00",
	"2006-01-02 15:04:05.999999",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

func (ft *FlexibleTime) UnmarshalJSON(data []byte) error {
	raw := strings.Trim(string(data), `"`)

	// null, "" and a missing value are all ordinary: plenty of Gamma
	// timestamps are simply not set. The zero time is the right answer, and an
	// error here would fail the whole event.
	if raw == "" || raw == "null" {
		ft.Time = time.Time{}
		return nil
	}

	for _, layout := range gammaTimeLayouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			ft.Time = parsed
			return nil
		}
	}

	// Still unrecognised: keep the zero value rather than failing the decode.
	// A missing timestamp costs one field; a returned error costs every event
	// in the response, which is the bug this type exists to prevent.
	ft.Time = time.Time{}
	return nil
}

func (ft FlexibleTime) MarshalJSON() ([]byte, error) {
	if ft.IsZero() {
		return []byte(`null`), nil
	}
	return []byte(`"` + ft.Format(time.RFC3339Nano) + `"`), nil
}
