// SPDX-License-Identifier: MIT

package budgeting

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/money"
)

// Pace projects where a budget will land by the end of its period, assuming
// spending continues at the rate observed so far. It answers "at this rate, will
// I stay within budget?" — the forward-looking complement to Status, which only
// reports spend to date.
type Pace struct {
	// Error reports invalid status arithmetic, if the supplied status is inconsistent.
	Error error
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

	limit, err := paceLimit(status)
	if err != nil {
		return Pace{Error: err}
	}

	projAmt := status.Spent.Amount
	if frac > 0 && frac < 1 {
		projAmt, err = money.MulDiv(status.Spent.Amount, int64(end.Sub(start)), int64(now.Sub(start)), money.RoundHalfAwayFromZero)
		if err != nil {
			return Pace{Elapsed: frac, Error: err}
		}
	}
	projected := money.New(projAmt, cur)

	overBy := money.Zero(cur)
	onTrack := true
	if projAmt > limit.Amount {
		overBy, err = projected.Sub(limit)
		if err != nil {
			return Pace{Elapsed: frac, Error: err}
		}
		onTrack = false
	}

	return Pace{
		Elapsed:   frac,
		Projected: projected,
		OverBy:    overBy,
		OnTrack:   onTrack,
	}
}

// PaceMarker locates the even-pace line for a budget: where spending SHOULD be
// right now if the discretionary limit were spent at a steady rate across the
// period, and how far actual spend is ahead of or behind it (BG3). It powers the
// second tick on the budget meter plus an "on pace" / "running $X hot" caption.
//
// Committed money (XC4 — recurring charges pre-spoken-for this period) is excluded
// from the race: it isn't "spent fast", it's already claimed, so the ideal line is
// drawn against the discretionary limit (limit − committed) only, and actual spend
// is likewise measured net of committed. The tick position (MarkerPct) is still
// expressed against the FULL limit so it lines up with the meter's overall scale.
type PaceMarker struct {
	// Error reports invalid status arithmetic, if the supplied status is inconsistent.
	Error error
	// Elapsed is the fraction of the period that has passed, in [0, 1].
	Elapsed float64
	// Ideal is the even-pace spend-to-date: discretionary limit × elapsed.
	Ideal money.Money
	// Delta is discretionary spend minus Ideal. Positive means spending is ahead
	// of pace ("hot"); negative means behind pace (a cushion so far).
	Delta money.Money
	// Hot reports whether discretionary spend has outrun the even-pace line
	// (Delta > 0) — the signal a row tones to ahead/behind-pace rather than only
	// over/under-limit.
	Hot bool
	// MarkerPct is Ideal as a percent of the full limit, clamped to [0, 100] — the
	// meter tick's left offset.
	MarkerPct int
}

// ProjectPaceMarker builds the PaceMarker from a budget's Status, its committed
// (XC4) amount, and the period bounds. Pass money.Zero(currency) for committed
// when the budget has no committed split. Before any time elapses the ideal line
// sits at zero (everything is "hot" if anything is spent); a degenerate period is
// treated as fully elapsed.
func ProjectPaceMarker(status Status, committed money.Money, start, end, now time.Time) PaceMarker {
	cur := status.Spent.Currency
	frac := elapsedFraction(start, end, now)

	limit, err := paceLimit(status)
	if err != nil {
		return PaceMarker{Error: err}
	}

	// Discretionary limit and discretionary spend both net out committed money.
	if committed.Currency != cur {
		return PaceMarker{Error: money.ErrCurrencyMismatch}
	}
	discLimit, err := money.Sub(limit.Amount, committed.Amount)
	if err != nil {
		return PaceMarker{Error: err}
	}
	if discLimit < 0 {
		discLimit = 0
	}
	discSpent, err := money.Sub(status.Spent.Amount, committed.Amount)
	if err != nil {
		return PaceMarker{Error: err}
	}
	if discSpent < 0 {
		discSpent = 0
	}

	idealAmt := discLimit
	if frac == 0 {
		idealAmt = 0
	} else if frac < 1 {
		idealAmt, err = money.MulDiv(discLimit, int64(now.Sub(start)), int64(end.Sub(start)), money.RoundHalfAwayFromZero)
		if err != nil {
			return PaceMarker{Error: err}
		}
	}
	delta, err := money.Sub(discSpent, idealAmt)
	if err != nil {
		return PaceMarker{Error: err}
	}

	markerPct := 0
	if limit.Amount > 0 {
		pct, err := money.MulDiv(idealAmt, 100, limit.Amount, money.RoundTowardZero)
		if err != nil {
			return PaceMarker{Error: err}
		}
		markerPct = int(pct)
	}
	if markerPct > 100 {
		markerPct = 100
	}
	if markerPct < 0 {
		markerPct = 0
	}

	return PaceMarker{
		Elapsed:   frac,
		Ideal:     money.New(idealAmt, cur),
		Delta:     money.New(delta, cur),
		Hot:       delta > 0,
		MarkerPct: markerPct,
	}
}

func paceLimit(status Status) (money.Money, error) {
	if status.Limit.Currency != "" {
		if status.Limit.Currency != status.Spent.Currency {
			return money.Money{}, money.ErrCurrencyMismatch
		}
		return status.Limit, nil
	}
	return status.Spent.Add(status.Remaining)
}
