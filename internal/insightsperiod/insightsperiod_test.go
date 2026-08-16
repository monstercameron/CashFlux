// SPDX-License-Identifier: MIT

package insightsperiod

import (
	"testing"
	"time"
)

// A Tuesday well inside a month, so partial-window behaviour is visible.
var now = time.Date(2026, time.August, 17, 14, 30, 0, 0, time.UTC)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestThisMonthRunsToTheEndOfTodayNotThisMorning(t *testing.T) {
	r := Resolve(ThisMonth, now)
	if !r.Start.Equal(day(2026, time.August, 1)) {
		t.Fatalf("start = %v", r.Start)
	}
	// A window ending at midnight this morning silently omits everything bought
	// today — the spending most likely being checked.
	if !r.End.Equal(day(2026, time.August, 18)) {
		t.Fatalf("end = %v, want the start of tomorrow so today counts", r.End)
	}
}

func TestAPartialMonthIsComparedWithTheSamePartOfTheMonthBefore(t *testing.T) {
	// Seventeen days of August compared with seventeen days of July, NOT with the
	// whole of July. This is the check that stops a normal month reading as a
	// collapse in spending.
	r := Resolve(ThisMonth, now)
	span := r.End.Sub(r.Start)
	priorSpan := r.PriorEnd.Sub(r.PriorStart)
	if span != priorSpan {
		t.Fatalf("window is %v but its comparison is %v — a partial period compared against a complete one", span, priorSpan)
	}
	if !r.PriorEnd.Equal(r.Start) {
		t.Fatalf("the comparison window does not end where the current one begins: %v vs %v", r.PriorEnd, r.Start)
	}
	if !r.PriorStart.Equal(day(2026, time.July, 15)) {
		t.Fatalf("prior start = %v, want 17 days before 1 August", r.PriorStart)
	}
}

func TestLastMonthIsTheCompleteMonth(t *testing.T) {
	r := Resolve(LastMonth, now)
	if !r.Start.Equal(day(2026, time.July, 1)) || !r.End.Equal(day(2026, time.August, 1)) {
		t.Fatalf("range = %v..%v", r.Start, r.End)
	}
	if !r.PriorStart.Equal(day(2026, time.June, 1)) || !r.PriorEnd.Equal(day(2026, time.July, 1)) {
		t.Fatalf("prior = %v..%v, want June", r.PriorStart, r.PriorEnd)
	}
}

func TestLongerPeriodsSpanTheRightNumberOfMonths(t *testing.T) {
	three := Resolve(Last3Months, now)
	if !three.Start.Equal(day(2026, time.June, 1)) {
		t.Fatalf("3-month start = %v, want 1 June (June, July, August so far)", three.Start)
	}
	twelve := Resolve(Last12Months, now)
	if !twelve.Start.Equal(day(2025, time.September, 1)) {
		t.Fatalf("12-month start = %v, want 1 September 2025", twelve.Start)
	}
}

func TestEveryPeriodComparesAgainstTheWindowImmediatelyBefore(t *testing.T) {
	// The invariant that holds for ALL periods: the comparison window ends exactly
	// where the current one begins, so the two never overlap and never leave a gap.
	// The LENGTH rule differs by kind — a partial window matches its comparison
	// day-for-day, while a complete calendar month is compared with the previous
	// calendar month even though months are unequal lengths.
	for _, p := range All() {
		r := Resolve(p, now)
		if !r.PriorEnd.Equal(r.Start) {
			t.Errorf("%s: comparison ends at %v but the window starts at %v", p, r.PriorEnd, r.Start)
		}
		if r.End.Before(r.Start) || r.PriorEnd.Before(r.PriorStart) {
			t.Errorf("%s: a window ends before it starts", p)
		}
		if p == LastMonth {
			continue
		}
		if got, want := r.PriorEnd.Sub(r.PriorStart), r.End.Sub(r.Start); got != want {
			t.Errorf("%s: comparison window is %v, current is %v — a partial period compared against a different length", p, got, want)
		}
	}
}

func TestACompleteMonthComparesWithTheMonthBeforeNotWithNDaysAgo(t *testing.T) {
	// July has 31 days and June has 30. Shifting July back by its own length lands
	// on 31 May — an off-by-a-day-and-a-half comparison nobody reading "vs last
	// month" would expect.
	r := Resolve(LastMonth, now)
	if !r.PriorStart.Equal(day(2026, time.June, 1)) {
		t.Fatalf("prior start = %v, want 1 June", r.PriorStart)
	}
}

func TestAnUnknownPeriodFallsBackToThisMonth(t *testing.T) {
	// A stale stored value must degrade to the default, not to an empty window
	// that would render a briefing about no time at all.
	r := Resolve(Period("last-decade"), now)
	if r.Period != ThisMonth {
		t.Fatalf("period = %q", r.Period)
	}
	if !r.Start.Equal(day(2026, time.August, 1)) {
		t.Fatalf("start = %v", r.Start)
	}
}

func TestValidRecognisesExactlyTheOfferedPeriods(t *testing.T) {
	for _, p := range All() {
		if !Valid(p) {
			t.Errorf("%s is offered but not valid", p)
		}
	}
	if Valid("") || Valid("nonsense") {
		t.Fatal("an unknown period reported as valid")
	}
}

func TestMonthsSizesATrendChart(t *testing.T) {
	for _, tc := range []struct {
		p    Period
		want int
	}{
		{ThisMonth, 1},
		{LastMonth, 1},
		{Last3Months, 3},
		{Last12Months, 12},
	} {
		if got := Resolve(tc.p, now).Months(); got != tc.want {
			t.Errorf("%s: Months() = %d, want %d", tc.p, got, tc.want)
		}
	}
}

func TestResolvingOnTheFirstOfAMonthIsSane(t *testing.T) {
	// The edge that breaks naive arithmetic: one day into the month.
	first := time.Date(2026, time.September, 1, 9, 0, 0, 0, time.UTC)
	r := Resolve(ThisMonth, first)
	if !r.Start.Equal(day(2026, time.September, 1)) || !r.End.Equal(day(2026, time.September, 2)) {
		t.Fatalf("range = %v..%v", r.Start, r.End)
	}
	if !r.PriorStart.Equal(day(2026, time.August, 31)) {
		t.Fatalf("prior start = %v, want the equivalent single day before", r.PriorStart)
	}
}

func TestEveryPeriodHasCopyKeys(t *testing.T) {
	for _, p := range All() {
		if LabelKey(p) == "" || ComparisonKey(p) == "" {
			t.Errorf("%s has no copy key", p)
		}
	}
}
