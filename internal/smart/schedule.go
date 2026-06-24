// SPDX-License-Identifier: MIT

package smart

import "time"

// Cadence controls WHEN a SMART feature runs — the per-feature schedule. It is
// the heart of making the layer feel like a configurable automation system
// rather than a fixed read-out:
//
//   - Free (deterministic) features are cheap and instant, so they default to
//     Live (always computed and shown).
//   - AI features cost money per call, so they default to Manual (run only when
//     the user clicks "Run") — the click-before-run guard — and can be scheduled
//     to auto-run on app open, when data changes, or on a daily/weekly/monthly
//     cadence, with the last result cached between runs.
type Cadence string

const (
	// CadenceLive recomputes every time the surface renders (Free default).
	CadenceLive Cadence = "live"
	// CadenceManual runs only on an explicit "Run now" (AI default — never spends
	// money on its own).
	CadenceManual Cadence = "manual"
	// CadenceOnOpen runs once per app open.
	CadenceOnOpen Cadence = "on_open"
	// CadenceOnChange runs when the underlying data changes (e.g. a new transaction).
	CadenceOnChange Cadence = "on_change"
	// CadenceDaily / Weekly / Monthly run on that fixed cadence.
	CadenceDaily   Cadence = "daily"
	CadenceWeekly  Cadence = "weekly"
	CadenceMonthly Cadence = "monthly"
)

// cadenceLabels are the human labels shown in the schedule picker.
var cadenceLabels = map[Cadence]string{
	CadenceLive:     "Always",
	CadenceManual:   "Manual",
	CadenceOnOpen:   "On app open",
	CadenceOnChange: "On new data",
	CadenceDaily:    "Daily",
	CadenceWeekly:   "Weekly",
	CadenceMonthly:  "Monthly",
}

// Valid reports whether c is a known cadence.
func (c Cadence) Valid() bool { _, ok := cadenceLabels[c]; return ok }

// Label returns the human label, or the raw value if unknown.
func (c Cadence) Label() string {
	if l, ok := cadenceLabels[c]; ok {
		return l
	}
	return string(c)
}

// AllCadences returns every cadence in picker order.
func AllCadences() []Cadence {
	return []Cadence{CadenceLive, CadenceManual, CadenceOnOpen, CadenceOnChange, CadenceDaily, CadenceWeekly, CadenceMonthly}
}

// DefaultCadence is the out-of-box schedule for a feature tier: Free runs Live
// (cheap, instant); AI runs Manual (don't spend money until asked).
func DefaultCadence(t Tier) Cadence {
	if t == TierAI {
		return CadenceManual
	}
	return CadenceLive
}

// period returns the minimum interval between runs for a time-based cadence, or
// 0 for non-interval cadences.
func (c Cadence) period() time.Duration {
	switch c {
	case CadenceDaily:
		return 24 * time.Hour
	case CadenceWeekly:
		return 7 * 24 * time.Hour
	case CadenceMonthly:
		return 30 * 24 * time.Hour
	default:
		return 0
	}
}

// Due reports whether a feature scheduled at cadence c is due to (re)run now,
// given when it last ran, the current time, whether the data has changed since
// the last run, and whether this is a fresh app open. Live is always due; Manual
// is never auto-due (it runs only on an explicit trigger). A never-run feature on
// any auto cadence is due so its first result appears.
func (c Cadence) Due(lastRun, now time.Time, dataChanged, appOpen bool) bool {
	switch c {
	case CadenceLive:
		return true
	case CadenceManual:
		return false
	case CadenceOnOpen:
		return appOpen || lastRun.IsZero()
	case CadenceOnChange:
		return dataChanged || lastRun.IsZero()
	case CadenceDaily, CadenceWeekly, CadenceMonthly:
		return lastRun.IsZero() || now.Sub(lastRun) >= c.period()
	default:
		return true
	}
}

// FreshFor reports whether a cached result produced at lastRun is still "fresh"
// for cadence c at time now — i.e. shown without re-running. Time-based cadences
// stay fresh for their period; Manual stays fresh until the user runs again
// (always fresh once produced); Live/OnOpen/OnChange are recomputed, so their
// freshness is governed by Due, and this returns true only when a result exists.
func (c Cadence) FreshFor(lastRun, now time.Time) bool {
	if lastRun.IsZero() {
		return false
	}
	if p := c.period(); p > 0 {
		return now.Sub(lastRun) < p
	}
	return true
}
