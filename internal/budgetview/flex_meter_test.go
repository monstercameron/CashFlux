// SPDX-License-Identifier: MIT

package budgetview

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/money"
)

func flexMeterView(spent, target int64) budgeting.FlexView {
	return budgeting.FlexView{Spent: money.New(spent, "USD"), Target: money.New(target, "USD"), Remaining: money.New(target-spent, "USD"), Over: spent > target}
}

func TestFlexMeterPreservesVerdictPrecedenceAndStrictThresholds(t *testing.T) {
	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	end, now := start.Add(100*time.Second), start.Add(50*time.Second)
	for _, tc := range []struct {
		name          string
		spent, target int64
		want          FlexPaceState
	}{
		{"unset zero", 100, 0, FlexPaceUnset}, {"unset negative", 100, -1, FlexPaceUnset},
		{"over first", 101, 100, FlexPaceOver}, {"tight before fast", 81, 100, FlexPaceTight},
		{"exact twenty minor left is not tight", 80, 100, FlexPaceFast},
		{"strict ten points ahead equality", 60, 100, FlexPaceOnTrack}, {"over ten points ahead", 61, 100, FlexPaceFast},
		{"zero spending", 0, 100, FlexPaceOnTrack}, {"negative spending", -10, 100, FlexPaceOnTrack},
		{"small target integer division", 4, 4, FlexPaceFast}, {"next small target tight", 5, 5, FlexPaceTight},
	} {
		t.Run(tc.name, func(t *testing.T) {
			view := flexMeterView(tc.spent, tc.target)
			before := view
			got := PrepareFlexMeter(view, tc.target, now, start, end)
			if got.State != tc.want || got.Unset != (tc.target <= 0) || !reflect.DeepEqual(view, before) {
				t.Fatalf("got %+v want%s; view mutated=%v", got, tc.want, !reflect.DeepEqual(view, before))
			}
			if math.IsNaN(got.FillPct) || math.IsInf(got.FillPct, 0) || got.FillPct < 0 || got.FillPct > 100 {
				t.Fatal("invalid fill", got)
			}
		})
	}
}

func TestFlexMeterPreservesPassedClockAndStrictTickBoundaries(t *testing.T) {
	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.FixedZone("offset", -5*3600))
	end := start.Add(100 * time.Second)
	for _, tc := range []struct {
		elapsed time.Duration
		percent float64
		tick    bool
	}{
		{-time.Second, 0, false}, {0, 0, false}, {3 * time.Second, 3, false}, {3*time.Second + time.Nanosecond, 3.000000001, true},
		{50 * time.Second, 50, true}, {97 * time.Second, 97, false}, {97*time.Second - time.Nanosecond, 96.999999999, true}, {100 * time.Second, 100, false}, {101 * time.Second, 100, false},
	} {
		got := PrepareFlexMeter(flexMeterView(0, 100), 100, start.Add(tc.elapsed), start, end)
		if math.Abs(got.PacePct-tc.percent) > 1e-8 || got.ShowPace != tc.tick {
			t.Fatalf("elapsed%s got%+v want%v tick%v", tc.elapsed, got, tc.percent, tc.tick)
		}
	}
	if got := PrepareFlexMeter(flexMeterView(0, 100), 100, start, start, start); got.PacePct != 0 || got.ShowPace {
		t.Fatal("degenerate range must preserve zero pace", got)
	}
}

func TestFlexMeterLargeGeometryAndIndependentCheckedReading(t *testing.T) {
	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		spent, target int64
		fill          float64
	}{{math.MaxInt64, 1, 100}, {math.MaxInt64 - 1, math.MaxInt64, 100}, {math.MinInt64, 1, 0}} {
		// Only geometry is under test here; no unchecked remaining subtraction.
		view := budgeting.FlexView{Spent: money.New(tc.spent, "USD"), Remaining: money.New(1, "USD")}
		got := PrepareFlexMeter(view, tc.target, start, start, start.Add(24*time.Hour))
		if got.FillPct != tc.fill || math.IsNaN(got.PacePct) || math.IsInf(got.FillPct, 0) {
			t.Fatal(got)
		}
	}
	view := budgeting.FlexView{Spent: money.New(1, "USD"), Remaining: money.New(math.MinInt64, "USD"), Over: true}
	got := PrepareFlexMeter(view, 100, start, start, start.Add(24*time.Hour))
	if !errors.Is(got.ReadingErr, money.ErrOverflow) || got.State != FlexPaceOver || got.DaysLeft != 1 || got.Reading.Amount != 0 {
		t.Fatalf("reading overflow hid independent facts or wrapped: %+v", got)
	}
}

func TestFlexDaysLeftPreservesElapsedHourCeiling(t *testing.T) {
	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		remaining time.Duration
		want      int
	}{{-time.Nanosecond, 0}, {0, 0}, {time.Nanosecond, 1}, {24 * time.Hour, 1}, {24*time.Hour + time.Nanosecond, 2}, {48 * time.Hour, 2}} {
		if got := FlexDaysLeft(start, start.Add(tc.remaining)); got != tc.want {
			t.Fatalf("remaining%s days%d want%d", tc.remaining, got, tc.want)
		}
	}
	// A wall-calendar two-day span crossing the spring change is 47 elapsed
	// hours, so it still reads two days rather than using a new calendar policy.
	from := time.Date(2026, 3, 7, 12, 0, 0, 0, time.FixedZone("before", -5*3600))
	to := time.Date(2026, 3, 9, 12, 0, 0, 0, time.FixedZone("after", -4*3600))
	if got := FlexDaysLeft(from, to); got != 2 {
		t.Fatal(got)
	}
}
