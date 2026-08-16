# Volume-spike baseline — design

**Date:** 2026-08-16
**Roadmap item:** 4.7 — "Real volume baseline", replacing the lifetime-volume ÷ 30 heuristic
**Status:** approved, ready for an implementation plan

---

## 1. Correction to the roadmap premise

Roadmap 4.7 and `PROJECT_STATUS.md` §4.8 both specify the fix as "a real
baseline from CLOB `prices-history`". **That is not possible.** The endpoint
returns only `{t, p}` — a timestamp and a price — at every interval:

```
GET https://clob.polymarket.com/prices-history?market=<tokenID>&interval=1d
{"history":[{"t":1786674793,"p":0.074}, ...]}
```

There is no volume field to build a volume baseline from. The "OHLCV" framing
in `docs/docs_MCP_SERVER_SPEC.md` §2.2 is wrong for the same reason: O/H/L/C
would have to be derived by bucketing a tick series, and V is simply absent.

The data that *does* fix this is already in hand. Gamma returns `volume24hr`,
`volume1wk` and `volume1mo` per market, and `internal/domain/polymarket/gamma_schema.go`
already decodes them as `Volume24Hr`, `Volume1Wk`, `Volume1Mo`. They are unused
at the exact line the heuristic runs.

**Consequence:** 4.7 needs no new tool and no new fetch. It is independent of
`get_market_history` (4.9), which remains worth building for price trajectory —
just not for this.

## 2. The defect

`internal/application/signal/service.go:146-151`:

```go
baseline := 10000.0
if market.VolumeNum > 0 {
    baseline = market.VolumeNum / 30.0
}
```

Three distinct faults, in increasing order of severity.

**The denominator is inflated.** `volumeNum` is lifetime volume and 30 is a
hardcoded guess at market age. Measured against live Gamma data on
`ballon-dor-winner-2026` (25 markets carrying volume), `volumeNum/30`
overstates real daily volume by a **median of 12.3×**:

| Market | heuristic/day | `volume1wk`/7 | overstatement |
|---|---|---|---|
| Harry Kane | 110,616 | 15,090 | 7.3× |
| Lionel Messi | 63,563 | 12,296 | 5.2× |
| Kylian Mbappé | 51,659 | 12,109 | 4.3× |

Those are the six largest markets by volume, and their ratios sit *below* the
12.3× median — the error is worse on the thinner markets, where a real spike is
also the most informative. That market was created 2025-09-22, roughly 11
months old rather than 30 days.

**It fails in the dangerous direction.** Since

```
velocity_change_pct = (recent − baseline) / baseline × 100
is_spike            = velocity_change_pct > 200
```

an inflated baseline *shrinks* velocity, so `IsSpike` under-fires. Against a
12× inflated baseline a genuine 3× volume spike reads as roughly −75% — it
looks like a volume drought. The engine is silently missing the events it
exists to catch.

**The two sides have incompatible units.** `ComputeVolumeSignal` sums *all 100
fetched trades* into `recentVol` with no time window, then compares that total
to a per-*day* baseline. On a hot market those 100 trades may span an hour; on
a quiet one, months. Correcting only the baseline number would leave a rate
compared against a non-rate.

**And `10000.0` is fabricated.** When volume is missing the code invents a
plausible-looking number that flows to the LLM as fact — against the standing
rule that the reasoning agent must never be handed numbers that aren't real.

## 3. Design

### Domain — `internal/domain/signal/signal.go`

Baseline selection is deterministic arithmetic, so it belongs in the domain
layer where it is testable without mocks or a clock.

```go
type VolumeInput struct {
    Volume24Hr float64 // recent: last 24h
    Volume1Wk  float64 // trailing-week total, for the baseline
    Lifetime   float64 // fallback numerator
    AgeDays    float64 // real age, supplied by the caller
}

func ComputeVolumeSignal(in VolumeInput) VolumeSignal
```

`ComputeVolumeSignal` no longer takes `[]polymarket.Trade` — volume is not
derived from trades at all now.

Both sides come from Gamma: **last-24h volume against the trailing-week daily
average**. Units match exactly (a day versus an average day), both figures are
complete and authoritative, and no extra HTTP call is needed. It is also immune
to the 100-trade fetch cap, which would otherwise truncate — and so
under-report — precisely the busiest markets.

The known cost is responsiveness: a 24h window means a fresh spike takes hours
to become visible. Accepted deliberately; a tighter trade-derived window trades
that for truncation bias on exactly the markets that matter most.

Baseline ladder:

| Order | Condition | Baseline | `BaselineSource` |
|---|---|---|---|
| 1 | `Volume1Wk > 0` | `Volume1Wk / 7` | `trailing_7d` |
| 2 | `Lifetime > 0 && AgeDays > 0` | `Lifetime / AgeDays` | `lifetime_avg` |
| 3 | otherwise | none | `Available: false` |

Step 2 uses the market's **real age**, never a hardcoded 30. Step 3 replaces
the `10000.0` fabrication: an absent signal is reported as absent.

`VolumeSignal` gains three fields:

```go
Available      bool   `json:"available"`
BaselineSource string `json:"baseline_source"` // trailing_7d | lifetime_avg
WindowHours    int    `json:"window_hours"`    // 24
```

`WindowHours` exists so the reasoning agent can see what "recent" means rather
than infer it, and `BaselineSource` so it can tell a real weekly average from
a degraded fallback.

### Application — `internal/application/signal/service.go`

Maps each market to a `VolumeInput`, computing `AgeDays` from `market.StartDate`
against the clock — keeping the domain free of both I/O and time.

The trade-fetch failure at line 129 stops being `continue`. With volume no
longer derived from trades, only the whale count depends on them, so
probability, orderbook skew and volume all remain reportable; the market is
returned with whales marked unavailable. This follows the repo's standing
"degraded-but-successful beats hard failure" convention, which the current
`continue` violates by dropping whole markets on a transient outage.

## 4. Testing

Table-driven domain tests, no mocks:

- healthy trailing-7d baseline
- spike correctly flagged above 3× a normal day
- fallback to lifetime ÷ real age when the week is empty
- both sources missing → `Available: false`, and no fabricated number
- `AgeDays == 0` → no divide-by-zero

Service-level: a market whose trade fetch fails still appears in the report,
carrying probability, skew and volume.

## 5. Out of scope

- **`get_market_history`** (roadmap 4.9) — independent, and cannot serve this.
- **The 200% spike threshold.** With units finally correct, "3× a normal day"
  is defensible. Changing the threshold in the same commit that changes what it
  measures would confound the two.

## 6. Downstream note

`recent_volume_usd` keeps its name but changes meaning: last-24h volume, not a
100-trade sum. Cygnus consumes this JSON, so the change needs flagging there.
`window_hours` is what makes the new meaning legible rather than silent.
