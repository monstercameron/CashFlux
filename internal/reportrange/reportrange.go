// SPDX-License-Identifier: MIT

// Package reportrange resolves a report's review window and the period it is
// compared against (C383).
//
// The annual review was a fixed twelve trailing months with automatic
// year-over-year, which answers exactly one question well and every other
// question not at all. "How did this quarter go" and "how does this half
// compare to the one before it" are the same report with two parameters
// changed, and both parameters were constants in a render function.
//
// Windows are always whole calendar months. Every figure in the report is
// bucketed by month — the flows, the savings-rate series, the category trends —
// so a range ending on the 17th would compare a partial month against whole
// ones and quietly understate the last bucket. A day-level range is a different
// report, not a parameter of this one.
package reportrange

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
)

// Preset names a review window.
type Preset string

const (
	// PresetTrailing12 is the twelve whole months ending with the anchor month —
	// the historical default, kept as the default.
	PresetTrailing12 Preset = "trailing-12"
	// PresetTrailing6 is the last six whole months.
	PresetTrailing6 Preset = "trailing-6"
	// PresetTrailing3 is the last three whole months.
	PresetTrailing3 Preset = "trailing-3"
	// PresetYearToDate is January of the anchor's year through the anchor month.
	PresetYearToDate Preset = "ytd"
	// PresetLastCalendarYear is the twelve months of the year before the anchor's.
	PresetLastCalendarYear Preset = "last-year"
	// PresetCustom is a caller-supplied month span.
	PresetCustom Preset = "custom"
)

// CompareMode names what the window is measured against.
type CompareMode string

const (
	// CompareSameLastYear is the same months one year earlier — the historical
	// default, and the right one for anything seasonal.
	CompareSameLastYear CompareMode = "last-year"
	// ComparePriorPeriod is the equally-long window immediately before this one.
	// It answers "is this better than last quarter", which seasonality makes a
	// different question from year-over-year.
	ComparePriorPeriod CompareMode = "prior"
	// CompareNone turns the comparison off. A report with nothing to compare
	// against is honest; one silently comparing against a window the reader did
	// not choose is not.
	CompareNone CompareMode = "none"
)

// MaxMonths bounds a window. Ten years of monthly buckets is past the point
// where a chart of them is readable, and the bound is what stops a custom range
// from asking the report to render a thousand columns.
const MaxMonths = 120

// Span is a half-open window of whole calendar months: Start is the first
// month's first day, End is the day AFTER the last month — the exclusive
// boundary every bucket loop in the report already expects.
type Span struct {
	Start, End time.Time
}

// Months is how many whole months the span covers.
func (s Span) Months() int {
	if s.Start.IsZero() || !s.Start.Before(s.End) {
		return 0
	}
	y := s.End.Year() - s.Start.Year()
	m := int(s.End.Month()) - int(s.Start.Month())
	return y*12 + m
}

// Valid reports whether the span covers at least one month and no more than
// MaxMonths.
func (s Span) Valid() bool {
	n := s.Months()
	return n >= 1 && n <= MaxMonths
}

// LastMonth is the first day of the span's final month — what a label wants,
// since End is one day into the month after.
func (s Span) LastMonth() time.Time { return dateutil.AddMonths(s.End, -1) }

// Shift moves the whole span by n months, preserving its length.
func (s Span) Shift(n int) Span {
	return Span{Start: dateutil.AddMonths(s.Start, n), End: dateutil.AddMonths(s.End, n)}
}

// spanEndingAt builds the months-long span whose last month is anchor's month.
func spanEndingAt(anchorMonth time.Time, months int) Span {
	end := dateutil.AddMonths(anchorMonth, 1)
	return Span{Start: dateutil.AddMonths(end, -months), End: end}
}

// Resolve turns a preset into a span. anchor is the month the report is
// currently sitting on (the top-bar period's last whole month); custom is used
// only by PresetCustom and is returned as-is after month-alignment.
//
// An unknown preset resolves to trailing twelve rather than to nothing: a
// preference written by a newer version must degrade to the default view, not to
// a blank report.
func Resolve(p Preset, anchor time.Time, custom Span) Span {
	a := dateutil.MonthStart(anchor)
	switch p {
	case PresetTrailing3:
		return spanEndingAt(a, 3)
	case PresetTrailing6:
		return spanEndingAt(a, 6)
	case PresetYearToDate:
		// January of the anchor's year through the anchor month inclusive. A
		// January anchor yields a one-month window, which is correct — the year
		// is one month old.
		start := time.Date(a.Year(), time.January, 1, 0, 0, 0, 0, a.Location())
		return Span{Start: start, End: dateutil.AddMonths(a, 1)}
	case PresetLastCalendarYear:
		start := time.Date(a.Year()-1, time.January, 1, 0, 0, 0, 0, a.Location())
		return Span{Start: start, End: time.Date(a.Year(), time.January, 1, 0, 0, 0, 0, a.Location())}
	case PresetCustom:
		s := Span{Start: dateutil.MonthStart(custom.Start), End: dateutil.MonthStart(custom.End)}
		if !s.Valid() {
			return spanEndingAt(a, 12)
		}
		return s
	}
	return spanEndingAt(a, 12)
}

// Compare returns the span the primary window is measured against, and whether
// there is one at all.
//
// Prior-period is the equally-long window immediately before, which is the
// honest reading of "versus last quarter": comparing three months against
// twelve, or against a differently-sized slice, would make every ratio in the
// report meaningless.
func Compare(primary Span, mode CompareMode) (Span, bool) {
	if !primary.Valid() {
		return Span{}, false
	}
	switch mode {
	case CompareSameLastYear:
		return primary.Shift(-12), true
	case ComparePriorPeriod:
		return primary.Shift(-primary.Months()), true
	}
	return Span{}, false
}

// Settings is a report's persisted window choice.
type Settings struct {
	Preset  Preset      `json:"preset,omitempty"`
	Compare CompareMode `json:"compare,omitempty"`
	// CustomStart / CustomEnd are "2006-01" month keys, used only by
	// PresetCustom. They are stored as months rather than dates so a reloaded
	// preference cannot reintroduce a partial month.
	CustomStart string `json:"customStart,omitempty"`
	CustomEnd   string `json:"customEnd,omitempty"`
}

// MonthKeyLayout is how a custom span's months are stored.
const MonthKeyLayout = "2006-01"

// Defaults is the historical behaviour: twelve trailing months versus the same
// months last year.
func Defaults() Settings {
	return Settings{Preset: PresetTrailing12, Compare: CompareSameLastYear}
}

// CustomSpan parses the stored month keys. The End key is INCLUSIVE as stored —
// a person picking "Jan to Mar" means three months — and is converted to the
// exclusive boundary here, in one place, rather than at each reader.
func (s Settings) CustomSpan() (Span, bool) {
	start, err := time.Parse(MonthKeyLayout, s.CustomStart)
	if err != nil {
		return Span{}, false
	}
	end, err := time.Parse(MonthKeyLayout, s.CustomEnd)
	if err != nil {
		return Span{}, false
	}
	sp := Span{Start: start, End: dateutil.AddMonths(end, 1)}
	if !sp.Valid() {
		return Span{}, false
	}
	return sp, true
}

// Windows resolves a settings value against an anchor month into the primary
// span, the comparison span, and whether the comparison exists.
//
// A PresetCustom whose stored months do not parse or do not form a valid span
// falls back to trailing twelve. A stored range is a preference, and a bad
// preference must degrade to the default report rather than to an error page.
func (s Settings) Windows(anchor time.Time) (Span, Span, bool) {
	custom, _ := s.CustomSpan()
	primary := Resolve(s.Preset, anchor, custom)
	cmp, ok := Compare(primary, s.Compare)
	return primary, cmp, ok
}
