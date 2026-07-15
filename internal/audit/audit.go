// SPDX-License-Identifier: MIT

// Package audit is the pure ranking layer behind the background auditor (AG6). It
// does NOT detect anything itself: the SMART Free engines (fee bleed, idle cash,
// price creep, dormant accounts, budget true-up, unbudgeted spending, earmark
// integrity, duplicate/anomaly detectors) already produce []smart.Insight. This
// package composes that combined output into a single PRIORITIZED findings list —
// ranked by dollar impact — where every row carries its evidence and a one-tap fix
// descriptor drawn from the insight's own Action.
//
// Keeping the ranking pure (no syscall/js, no model, no network) means the whole
// audit is deterministic and unit-tests on native Go; the wasm layer only gathers
// the insights (via smartengine.Run) and renders the Report.
package audit

import (
	"sort"

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

// Finding is one prioritized audit result: the underlying insight, the family it
// belongs to (its page, for grouping), the ranking impact in base-currency minor
// units, and the one-tap fix.
type Finding struct {
	Insight smart.Insight
	// Family is the human family label (the insight's page — "Accounts",
	// "Budgets", …), used to group findings in the card.
	Family string
	// ImpactMinor is the dollar magnitude used for ranking, in base-currency
	// minor units. It is the absolute value of the insight's headline Amount when
	// present, else 0 (a finding with no figure still ranks, below every priced
	// one, broken by severity).
	ImpactMinor int64
	Fix         Fix
}

// Report is the full prioritized audit: findings ranked most-impactful first, the
// summed dollar impact, and the base currency the impact is expressed in.
type Report struct {
	Findings         []Finding
	TotalImpactMinor int64
	Base             string
}

// TotalImpact returns the summed dollar impact as a money value in the base
// currency — the "Found $340/yr" headline figure.
func (r Report) TotalImpact() money.Money { return money.New(r.TotalImpactMinor, r.Base) }

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

// Audit ranks the combined detector output into a prioritized Report. Insights
// are ordered by dollar impact (descending); ties break by severity (higher
// first) then by Key so the order is stable and deterministic. base is the
// household base currency the insight amounts are already expressed in, carried
// through so the caller can format the total.
//
// Audit reads the insights read-only and never mutates them.
func Audit(insights []smart.Insight, base string) Report {
	findings := make([]Finding, 0, len(insights))
	var total int64
	for _, in := range insights {
		impact := int64(0)
		if in.HasAmount {
			impact = in.Amount.Amount
			if impact < 0 {
				impact = -impact
			}
		}
		total += impact
		findings = append(findings, Finding{
			Insight:     in,
			Family:      familyLabel(in),
			ImpactMinor: impact,
			Fix:         fixFor(in),
		})
	}
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.ImpactMinor != b.ImpactMinor {
			return a.ImpactMinor > b.ImpactMinor
		}
		if a.Insight.Severity != b.Insight.Severity {
			return a.Insight.Severity > b.Insight.Severity
		}
		return a.Insight.Key < b.Insight.Key
	})
	return Report{Findings: findings, TotalImpactMinor: total, Base: base}
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
