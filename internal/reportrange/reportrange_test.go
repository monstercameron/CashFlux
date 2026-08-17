// SPDX-License-Identifier: MIT

package reportrange

import (
	"testing"
	"time"
)

func m(y int, mo time.Month) time.Time { return time.Date(y, mo, 1, 0, 0, 0, 0, time.UTC) }

// The anchor is a mid-month day on purpose: every window must snap to whole
// months, or the last bucket compares a partial month against whole ones.
var anchor = time.Date(2026, time.August, 17, 14, 0, 0, 0, time.UTC)

func TestResolvePresets(t *testing.T) {
	cases := []struct {
		preset     Preset
		start, end time.Time
		months     int
	}{
		{PresetTrailing12, m(2025, time.September), m(2026, time.September), 12},
		{PresetTrailing6, m(2026, time.March), m(2026, time.September), 6},
		{PresetTrailing3, m(2026, time.June), m(2026, time.September), 3},
		{PresetYearToDate, m(2026, time.January), m(2026, time.September), 8},
		{PresetLastCalendarYear, m(2025, time.January), m(2026, time.January), 12},
		// An unrecognised preset must degrade to the default report, not to a
		// blank one — a preference written by a newer version will land here.
		{Preset("from-the-future"), m(2025, time.September), m(2026, time.September), 12},
	}
	for _, c := range cases {
		got := Resolve(c.preset, anchor, Span{})
		if !got.Start.Equal(c.start) || !got.End.Equal(c.end) {
			t.Errorf("%s = %s..%s, want %s..%s", c.preset,
				got.Start.Format("2006-01"), got.End.Format("2006-01"),
				c.start.Format("2006-01"), c.end.Format("2006-01"))
		}
		if got.Months() != c.months {
			t.Errorf("%s months = %d, want %d", c.preset, got.Months(), c.months)
		}
	}
}

// A January anchor makes year-to-date one month long. That is correct: the year
// is one month old, and rounding it up to twelve would report eleven months of
// a year that has not happened.
func TestYearToDateInJanuaryIsOneMonth(t *testing.T) {
	got := Resolve(PresetYearToDate, m(2026, time.January), Span{})
	if got.Months() != 1 {
		t.Errorf("months = %d, want 1", got.Months())
	}
	if !got.Start.Equal(m(2026, time.January)) {
		t.Errorf("start = %s", got.Start.Format("2006-01"))
	}
}

func TestResolveCustomSnapsToMonthsAndFallsBackWhenInvalid(t *testing.T) {
	custom := Span{
		Start: time.Date(2026, time.February, 14, 9, 0, 0, 0, time.UTC),
		End:   time.Date(2026, time.May, 30, 23, 0, 0, 0, time.UTC),
	}
	got := Resolve(PresetCustom, anchor, custom)
	if !got.Start.Equal(m(2026, time.February)) || !got.End.Equal(m(2026, time.May)) {
		t.Errorf("custom = %s..%s, want 2026-02..2026-05",
			got.Start.Format("2006-01"), got.End.Format("2006-01"))
	}
	// Backwards, empty, and absurdly long spans all fall back to the default.
	for _, bad := range []Span{
		{Start: m(2026, time.May), End: m(2026, time.February)},
		{},
		{Start: m(1900, time.January), End: m(2026, time.January)},
	} {
		if got := Resolve(PresetCustom, anchor, bad); got.Months() != 12 {
			t.Errorf("bad custom %+v resolved to %d months, want the 12-month default", bad, got.Months())
		}
	}
}

// Prior-period must be the SAME LENGTH: comparing three months against twelve
// would make every ratio in the report meaningless.
func TestComparePriorPeriodMatchesLength(t *testing.T) {
	primary := Resolve(PresetTrailing3, anchor, Span{})
	cmp, ok := Compare(primary, ComparePriorPeriod)
	if !ok {
		t.Fatal("no comparison")
	}
	if cmp.Months() != primary.Months() {
		t.Errorf("comparison is %d months against a %d-month window", cmp.Months(), primary.Months())
	}
	if !cmp.End.Equal(primary.Start) {
		t.Errorf("comparison ends %s, want it to abut the primary start %s",
			cmp.End.Format("2006-01"), primary.Start.Format("2006-01"))
	}
}

func TestCompareSameLastYearShiftsTwelve(t *testing.T) {
	primary := Resolve(PresetTrailing6, anchor, Span{})
	cmp, ok := Compare(primary, CompareSameLastYear)
	if !ok {
		t.Fatal("no comparison")
	}
	if !cmp.Start.Equal(m(2025, time.March)) || !cmp.End.Equal(m(2025, time.September)) {
		t.Errorf("comparison = %s..%s", cmp.Start.Format("2006-01"), cmp.End.Format("2006-01"))
	}
}

func TestCompareNone(t *testing.T) {
	primary := Resolve(PresetTrailing12, anchor, Span{})
	if _, ok := Compare(primary, CompareNone); ok {
		t.Error("CompareNone produced a comparison")
	}
	// An invalid primary has nothing to compare against either.
	if _, ok := Compare(Span{}, CompareSameLastYear); ok {
		t.Error("an empty primary produced a comparison")
	}
}

// The stored end month is INCLUSIVE — a person picking Jan to Mar means three
// months — and that conversion happens in one place.
func TestCustomSpanTreatsTheStoredEndAsInclusive(t *testing.T) {
	s := Settings{Preset: PresetCustom, CustomStart: "2026-01", CustomEnd: "2026-03"}
	sp, ok := s.CustomSpan()
	if !ok {
		t.Fatal("CustomSpan rejected a valid range")
	}
	if sp.Months() != 3 {
		t.Errorf("months = %d, want 3", sp.Months())
	}
	if !sp.End.Equal(m(2026, time.April)) {
		t.Errorf("end = %s, want the exclusive 2026-04", sp.End.Format("2006-01"))
	}
	// One month is a legal range.
	one := Settings{CustomStart: "2026-05", CustomEnd: "2026-05"}
	if sp, ok := one.CustomSpan(); !ok || sp.Months() != 1 {
		t.Errorf("a single-month range = %+v,%v", sp, ok)
	}
}

func TestCustomSpanRejectsGarbage(t *testing.T) {
	for _, s := range []Settings{
		{CustomStart: "", CustomEnd: "2026-03"},
		{CustomStart: "2026-01", CustomEnd: "nope"},
		{CustomStart: "2026-06", CustomEnd: "2026-01"},
	} {
		if _, ok := s.CustomSpan(); ok {
			t.Errorf("%+v was accepted", s)
		}
	}
}

// A bad stored preference must degrade to the default report rather than an
// error: the user cannot fix a settings blob, and a blank report is not a fix.
func TestWindowsFallsBackOnABrokenCustomRange(t *testing.T) {
	s := Settings{Preset: PresetCustom, Compare: CompareSameLastYear, CustomStart: "not-a-month"}
	primary, cmp, ok := s.Windows(anchor)
	if primary.Months() != 12 {
		t.Errorf("primary = %d months, want the 12-month default", primary.Months())
	}
	if !ok || cmp.Months() != 12 {
		t.Errorf("comparison = %+v,%v", cmp, ok)
	}
}

func TestDefaultsAreTheHistoricalBehaviour(t *testing.T) {
	primary, cmp, ok := Defaults().Windows(anchor)
	if primary.Months() != 12 || !ok {
		t.Fatalf("primary = %d months, cmp ok %v", primary.Months(), ok)
	}
	if !cmp.Start.Equal(primary.Start.AddDate(-1, 0, 0)) {
		t.Errorf("comparison start = %s", cmp.Start.Format("2006-01"))
	}
}

func TestSpanHelpers(t *testing.T) {
	sp := Resolve(PresetTrailing12, anchor, Span{})
	if !sp.LastMonth().Equal(m(2026, time.August)) {
		t.Errorf("LastMonth = %s, want 2026-08", sp.LastMonth().Format("2006-01"))
	}
	if got := sp.Shift(-12); !got.Start.Equal(m(2024, time.September)) {
		t.Errorf("Shift start = %s", got.Start.Format("2006-01"))
	}
	// A zero span has no months and is not valid — callers must not divide by it.
	if (Span{}).Months() != 0 || (Span{}).Valid() {
		t.Error("the zero Span claimed to be a window")
	}
	// A backwards span is zero months, not a negative count.
	back := Span{Start: m(2026, time.June), End: m(2026, time.January)}
	if back.Months() != 0 {
		t.Errorf("backwards span = %d months, want 0", back.Months())
	}
}
