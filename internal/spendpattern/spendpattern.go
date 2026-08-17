// SPDX-License-Identifier: MIT

// Package spendpattern finds habits: spending that runs consistently higher
// inside some phase of the calendar than outside it (WF-SM2).
//
// "Spend rises after payday" and "weekend delivery is accelerating" are the same
// question asked twice — is money going out faster during THESE days than during
// the others, again and again — so they are one engine rather than two
// detectors.
//
// # Rates, never totals
//
// The trap this package exists to avoid: there are five weekdays and two weekend
// days, so comparing TOTALS finds "you spend more on weekdays" in every
// household that has ever existed, forever, and calls it a pattern. Compare only
// ever divides by the number of days on each side, and the API takes days rather
// than sums so a caller cannot hand it totals by accident.
//
// # A pattern is a repeated thing
//
// One expensive weekend is not a habit, it is an anomaly — and the app already
// detects those. A finding therefore has to hold in most of the periods
// observed, not merely in the pooled average, which one extreme month can carry
// on its own. Without that rule this package would compete with the anomaly
// detector and lose, because a one-off dressed as a habit invites somebody to
// change a routine that was never the problem.
package spendpattern

import (
	"sort"
	"time"
)

// MinDaysEachSide is how many days each side needs before a comparison means
// anything. Eight — enough that a single day cannot set the rate, and low enough
// that a month of weekends (eight or nine days) qualifies.
const MinDaysEachSide = 8

// MinPeriods is how many periods must carry data on both sides before
// consistency can be judged. Three: two agreeing periods is a coincidence with
// an even chance of happening by itself.
const MinPeriods = 3

// MinHoldRatio is the share of periods in which the effect must actually appear.
//
// Two thirds. Demanding every period would discard real habits over one holiday
// month; a simple majority would call a coin-flip a pattern.
const MinHoldRatio = 2.0 / 3.0

// MinLiftPct is how much higher the inside rate must run to be worth saying.
//
// Twenty percent. Below that the claim rests on the difference between two
// averages of noisy daily spending, and telling somebody their habit shifted by
// a rounding error costs more trust than the observation is worth.
const MinLiftPct = 20.0

// Day is one day's spending, as a magnitude.
type Day struct {
	Date  time.Time
	Minor int64
}

// Finding is what the comparison concluded.
type Finding struct {
	// Known is false when there was not enough on one side to compare at all.
	// Reported rather than a zero lift, because "no pattern" and "we could not
	// look" are different answers and only the first is a finding.
	Known bool
	// InsideDays and OutsideDays are how many days each rate averages over — the
	// evidence behind the claim, and the first thing a sceptical reader asks for.
	InsideDays, OutsideDays int
	// InsideRateMinor and OutsideRateMinor are spending per day on each side.
	InsideRateMinor, OutsideRateMinor int64
	// LiftPct is how much higher the inside rate runs, as a percentage of the
	// outside rate. Negative when the inside is quieter.
	LiftPct float64
	// Periods is how many periods carried data on both sides; PeriodsHeld is how
	// many of those actually showed the effect.
	Periods, PeriodsHeld int
	// Consistent is whether the effect appeared in enough of those periods to be
	// a habit rather than one loud month.
	Consistent bool
}

// Holds reports whether this is worth telling somebody about: a real effect,
// large enough to act on, that keeps happening.
func (f Finding) Holds() bool {
	return f.Known && f.LiftPct >= MinLiftPct && f.Consistent
}

// Compare measures spending inside a phase against spending outside it.
//
// periodOf groups days into the spans that consistency is judged over — usually
// the month. A nil periodOf pools everything into one period, which disables the
// consistency check, so callers that care must supply it.
func Compare(inside, outside []Day, periodOf func(time.Time) string) Finding {
	var f Finding
	if len(inside) < MinDaysEachSide || len(outside) < MinDaysEachSide {
		return f // not enough to compare; say so rather than reporting no lift
	}
	f.Known = true
	f.InsideDays, f.OutsideDays = len(inside), len(outside)
	f.InsideRateMinor = rate(inside)
	f.OutsideRateMinor = rate(outside)
	f.LiftPct = lift(f.InsideRateMinor, f.OutsideRateMinor)

	if periodOf == nil {
		// One pooled period: there is nothing to be consistent across, and the
		// pooled average alone is exactly the evidence this package refuses to
		// treat as a habit.
		return f
	}
	inBy := byPeriod(inside, periodOf)
	outBy := byPeriod(outside, periodOf)
	keys := make([]string, 0, len(inBy))
	for k := range inBy {
		if _, ok := outBy[k]; ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys) // deterministic, though only the counts are used
	for _, k := range keys {
		f.Periods++
		if rate(inBy[k]) > rate(outBy[k]) {
			f.PeriodsHeld++
		}
	}
	f.Consistent = f.Periods >= MinPeriods &&
		float64(f.PeriodsHeld) >= float64(f.Periods)*MinHoldRatio
	return f
}

// ExtraPerDayMinor is how much more goes out on an inside day than an outside
// one. It is the figure worth showing next to the percentage: "37% higher" is an
// abstraction, and "$18 a day more" is a decision.
func (f Finding) ExtraPerDayMinor() int64 {
	if !f.Known {
		return 0
	}
	return f.InsideRateMinor - f.OutsideRateMinor
}

// rate is average spend per day, as a magnitude.
func rate(days []Day) int64 {
	if len(days) == 0 {
		return 0
	}
	var sum int64
	for _, d := range days {
		sum += abs64(d.Minor)
	}
	return sum / int64(len(days))
}

// lift is the inside rate as a percentage above the outside rate. An outside
// rate of zero yields no lift rather than an infinite one: spending nothing
// outside a phase is a fact about the phase being the only time money moves, and
// "infinitely more" is not a sentence anybody can act on.
func lift(in, out int64) float64 {
	if out <= 0 {
		return 0
	}
	return (float64(in) - float64(out)) / float64(out) * 100
}

func byPeriod(days []Day, periodOf func(time.Time) string) map[string][]Day {
	out := map[string][]Day{}
	for _, d := range days {
		k := periodOf(d.Date)
		out[k] = append(out[k], d)
	}
	return out
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// SplitBy divides a run of days into the ones matching a phase and the rest.
// It exists so callers describe the PHASE and never the arithmetic.
func SplitBy(days []Day, in func(time.Time) bool) (inside, outside []Day) {
	for _, d := range days {
		if in(d.Date) {
			inside = append(inside, d)
		} else {
			outside = append(outside, d)
		}
	}
	return inside, outside
}

// IsWeekend reports whether a date falls on Saturday or Sunday.
//
// The weekend is not user-configurable here on purpose: this is about when
// somebody is off work and ordering dinner, not about which day a week starts on
// in their calendar preference, and conflating the two would make the finding
// mean different things to different households.
func IsWeekend(t time.Time) bool {
	switch t.Weekday() {
	case time.Saturday, time.Sunday:
		return true
	}
	return false
}

// WithinDaysAfter returns a phase test for the N days following any of the given
// dates — payday being the one that matters.
//
// The day itself counts as inside: money that goes out the moment it arrives is
// the whole point of the question.
func WithinDaysAfter(marks []time.Time, days int) func(time.Time) bool {
	if days < 0 {
		days = 0
	}
	keys := make(map[string]bool, len(marks)*(days+1))
	for _, m := range marks {
		for i := 0; i <= days; i++ {
			keys[m.AddDate(0, 0, i).Format("2006-01-02")] = true
		}
	}
	return func(t time.Time) bool { return keys[t.Format("2006-01-02")] }
}

// MonthKey groups days by calendar month, the usual span for consistency.
func MonthKey(t time.Time) string { return t.Format("2006-01") }
