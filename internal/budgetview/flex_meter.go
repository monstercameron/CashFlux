// SPDX-License-Identifier: MIT

package budgetview

import (
	"math"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/money"
)

// FlexPaceState identifies the flex meter's existing ordered verdict policy.
type FlexPaceState string

const (
	// FlexPaceUnset invites a target before showing financial readings.
	FlexPaceUnset FlexPaceState = "unset"
	// FlexPaceOver means the evaluated view exceeded its target.
	FlexPaceOver FlexPaceState = "over"
	// FlexPaceTight means remaining minor units are strictly below target/5.
	FlexPaceTight FlexPaceState = "tight"
	// FlexPaceFast means spending is over ten percentage points ahead of time.
	FlexPaceFast FlexPaceState = "fast"
	// FlexPaceOnTrack is the default after all stronger verdicts are excluded.
	FlexPaceOnTrack FlexPaceState = "on-track"
)

// FlexMeter contains unformatted geometry, clock facts and a checked reading.
// Reading is remaining money, or its positive magnitude when the view is over.
// ReadingErr affects only that amount, not the independent meter/clock facts.
type FlexMeter struct {
	Unset            bool
	FillPct, PacePct float64
	ShowPace         bool
	State            FlexPaceState
	DaysLeft         int
	Reading          money.Money
	ReadingErr       error
}

// PrepareFlexMeter preserves the flex-specific float geometry and strict
// thresholds. It does not recompute spending/remaining or alter the evaluated
// view. The caller supplies current reference time outside expensive view caches.
func PrepareFlexMeter(view budgeting.FlexView, target int64, now, start, end time.Time) FlexMeter {
	if target <= 0 {
		return FlexMeter{Unset: true, State: FlexPaceUnset}
	}
	m := FlexMeter{State: FlexPaceOnTrack, Reading: view.Remaining}
	spent, limit := float64(view.Spent.Amount), float64(target)
	m.FillPct = min(100, max(0, spent/limit*100))
	var pace float64
	if span := end.Sub(start); span > 0 {
		pace = now.Sub(start).Seconds() / span.Seconds()
	}
	pace = min(1, max(0, pace))
	m.PacePct, m.ShowPace = pace*100, pace > 0.03 && pace < 0.97
	m.DaysLeft = FlexDaysLeft(now, end)
	switch {
	case view.Over:
		m.State = FlexPaceOver
		m.Reading, m.ReadingErr = view.Remaining.AbsChecked()
	case view.Remaining.Amount < target/5:
		m.State = FlexPaceTight
	case spent/limit > pace+0.1:
		m.State = FlexPaceFast
	}
	return m
}

// FlexDaysLeft rounds positive remaining elapsed hours up to 24-hour units,
// including the current partial day. It intentionally does not count calendar
// boundaries; zero means the period has ended and its caption should be hidden.
func FlexDaysLeft(now, end time.Time) int {
	hours := end.Sub(now).Hours()
	if hours <= 0 {
		return 0
	}
	return int(math.Ceil(hours / 24))
}
