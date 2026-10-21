// SPDX-License-Identifier: MIT

// Package audit is the pure ranking layer behind the background auditor (AG6). It
// does NOT detect anything itself: the SMART Free engines (fee bleed, idle cash,
// price creep, dormant accounts, budget true-up, unbudgeted spending, earmark
// integrity, duplicate/anomaly detectors) already produce []smart.Insight. This
// package composes that combined output into a single PRIORITIZED findings list —
// grouped by compatible impact periods — where every row carries its evidence and a one-tap fix
// descriptor drawn from the insight's own Action.
//
// Keeping the ranking pure (no syscall/js, no model, no network) means the whole
// audit is deterministic and unit-tests on native Go; the wasm layer only gathers
// the insights (via smartengine.Run) and renders the Report.
package audit

import (
	"fmt"
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/currency"

	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
)

// Fix is the one-tap remedy descriptor for a finding, distilled from the
// insight's optional Action so the UI can render a single actionable button
// without re-deriving anything. OneTap is true when the finding carries an
// action the user can apply directly (as opposed to an info-only observation).
type Fix struct {
	// Label is the button text ("Cancel subscription", "Move idle cash"). Empty
	// when the finding is informational.
	Label string
	// Kind mirrors the insight's ActionKind ("cancel_subscription", "navigate",
	// …), so the host dispatches the same handler the Smart strip uses.
	Kind smart.ActionKind
	// Route is the screen the fix opens, when the action navigates.
	Route string
	// OneTap reports whether an actionable fix is attached.
	OneTap bool
}

// Finding retains the detector evidence and its checked base-currency amount.
// Amounts are comparable only within the same period group.
type Finding struct {
	Insight     smart.Insight
	Family      string
	ImpactMinor int64
	// ComparisonMinor annualizes only explicitly recurring impacts for ranking.
	ComparisonMinor int64
	Fix             Fix
}

// Report separates recurring annual estimates, one-time estimates, and context
// amounts whose impact cadence the detector did not classify. These groups must
// not be added together or presented as guaranteed independent savings.
type Report struct {
	Findings             []Finding
	Base                 string
	RecurringAnnualMinor int64
	OneTimeMinor         int64
	UnclassifiedCount    int
	// Err is set only by the compatibility Audit wrapper; failed reports have no findings.
	Err error
}

// OneTapCount returns how many findings carry an actionable one-tap fix.
func (r Report) OneTapCount() int {
	n := 0
	for _, f := range r.Findings {
		if f.Fix.OneTap {
			n++
		}
	}
	return n
}

// Audit retains the original entry point for callers without a conversion table.
// Foreign currency or arithmetic failures remain explicit through Report.Err.
func Audit(insights []smart.Insight, base string) Report {
	r, err := AuditChecked(insights, base, currency.Rates{Base: base})
	if err != nil {
		return Report{Base: base, Err: err}
	}
	return r
}

// AuditChecked converts headline amounts using current rates and ranks findings
// within compatible period groups. Unknown-cadence figures remain context, not
// savings; one-time impacts are never added to recurring annual estimates.
func AuditChecked(insights []smart.Insight, base string, rates currency.Rates) (Report, error) {
	base = strings.ToUpper(strings.TrimSpace(base))
	if base == "" {
		return Report{}, fmt.Errorf("audit base currency is required")
	}
	r := Report{Base: base, Findings: make([]Finding, 0, len(insights))}
	for _, in := range insights {
		f := Finding{Insight: in, Family: familyLabel(in), Fix: fixFor(in)}
		if in.HasAmount {
			if strings.TrimSpace(in.Amount.Currency) == "" {
				return Report{}, fmt.Errorf("audit finding %q has no amount currency", in.Key)
			}
			converted, err := rates.Convert(in.Amount, base)
			if err != nil {
				return Report{}, fmt.Errorf("audit finding %q conversion: %w", in.Key, err)
			}
			f.ImpactMinor, err = money.Abs(converted.Amount)
			if err != nil {
				return Report{}, fmt.Errorf("audit finding %q magnitude: %w", in.Key, err)
			}
			f.ComparisonMinor = f.ImpactMinor
			switch in.AmountCadence {
			case smart.AmountMonthly:
				f.ComparisonMinor, err = money.Mul(f.ImpactMinor, 12)
				if err == nil {
					r.RecurringAnnualMinor, err = money.Add(r.RecurringAnnualMinor, f.ComparisonMinor)
				}
			case smart.AmountAnnual:
				r.RecurringAnnualMinor, err = money.Add(r.RecurringAnnualMinor, f.ImpactMinor)
			case smart.AmountOneTime:
				r.OneTimeMinor, err = money.Add(r.OneTimeMinor, f.ImpactMinor)
			default:
				r.UnclassifiedCount++
			}
			if err != nil {
				return Report{}, fmt.Errorf("audit finding %q period impact: %w", in.Key, err)
			}
		}
		r.Findings = append(r.Findings, f)
	}
	sort.SliceStable(r.Findings, func(i, j int) bool {
		a, b := r.Findings[i], r.Findings[j]
		ga, gb := impactGroup(a), impactGroup(b)
		if ga != gb {
			return ga < gb
		}
		// Unclassified headline amounts may describe a balance or shortfall rather
		// than an action's savings, so they rank by severity rather than money.
		if ga < 2 && a.ComparisonMinor != b.ComparisonMinor {
			return a.ComparisonMinor > b.ComparisonMinor
		}
		if a.Insight.Severity != b.Insight.Severity {
			return a.Insight.Severity > b.Insight.Severity
		}
		return a.Insight.Key < b.Insight.Key
	})
	return r, nil
}

func impactGroup(f Finding) int {
	if !f.Insight.HasAmount {
		return 3
	}
	switch f.Insight.AmountCadence {
	case smart.AmountMonthly, smart.AmountAnnual:
		return 0
	case smart.AmountOneTime:
		return 1
	default:
		return 2
	}
}

// familyLabel returns the grouping label for an insight — its page's human name,
// falling back to "General" when the page is unset/unknown.
func familyLabel(in smart.Insight) string {
	if in.Page.Valid() {
		return in.Page.Label()
	}
	return "General"
}

// fixFor distills the one-tap fix from an insight's Action. An info-only insight
// (no action) yields an empty, non-one-tap Fix.
func fixFor(in smart.Insight) Fix {
	if in.Action == nil || in.Action.Kind == smart.ActionNone {
		return Fix{}
	}
	a := in.Action
	return Fix{Label: a.Label, Kind: a.Kind, Route: a.Route, OneTap: true}
}
