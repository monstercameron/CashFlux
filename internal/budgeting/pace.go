package budgeting

import (
	"math"
	"time"

	"github.com/monstercameron/CashFlux/internal/money"
)

// Pace projects where a budget will land by the end of its period, assuming
// spending continues at the rate observed so far. It answers "at this rate, will
// I stay within budget?" — the forward-looking complement to Status, which only
// reports spend to date.
type Pace struct {
	// Elapsed is the fraction of the period that has passed, in [0, 1].
	Elapsed float64
	// Projected is the spend forecast for the whole period at the current rate.
	Projected money.Money
	// OverBy is the projected overspend (Projected − limit), or zero when the
	// projection stays within the limit.
	OverBy money.Money
	// OnTrack reports whether the projection lands within the limit.
	OnTrack bool
}

// elapsedFraction returns how far now is through the half-open period
// [start, end), clamped to [0, 1]. A non-positive span yields 1 (treat a
// degenerate period as fully elapsed).
func elapsedFraction(start, end, now time.Time) float64 {
	span := end.Sub(start)
	if span <= 0 {
		return 1
	}
	if !now.After(start) {
		return 0
	}
	if !now.Before(end) {
		return 1
	}
	return float64(now.Sub(start)) / float64(span)
}

// ProjectPace forecasts a budget's end-of-period spend from its Status and the
// period bounds. It extrapolates linearly: projected = spent ÷ fraction-elapsed.
// Before any time has elapsed (or for a degenerate period) it can't extrapolate,
// so it reports the spend so far as the projection. The limit is recovered from
// the Status (Spent + Remaining), so no rate table is needed and the currency
// always matches Status.Spent.
//
// Linear extrapolation is noisy very early in a period — a single large purchase
// on day one projects to a huge total — so callers should present the projection
// as a gentle heads-up, not a hard prediction.
func ProjectPace(status Status, start, end, now time.Time) Pace {
	cur := status.Spent.Currency
	frac := elapsedFraction(start, end, now)

	limit, _ := status.Spent.Add(status.Remaining) // limit = spent + (limit − spent)

	projAmt := status.Spent.Amount
	if frac > 0 {
		f := float64(status.Spent.Amount) / frac
		if f > math.MaxInt64 {
			f = math.MaxInt64 // guard a tiny fraction from overflowing int64
		}
		projAmt = int64(math.Round(f))
	}
	projected := money.New(projAmt, cur)

	overBy := money.Zero(cur)
	onTrack := true
	if projAmt > limit.Amount {
		overBy = money.New(projAmt-limit.Amount, cur)
		onTrack = false
	}

	return Pace{
		Elapsed:   frac,
		Projected: projected,
		OverBy:    overBy,
		OnTrack:   onTrack,
	}
}
