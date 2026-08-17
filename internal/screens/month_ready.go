// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgetplan"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/monthready"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// unbudgetedLookbackMonths is how far back a category must have been used before
// its lack of a budget counts as a gap.
//
// Three months. A category somebody used once last winter does not need a budget
// for next month, and listing it would turn a short actionable list into a
// catalogue of everything they have ever bought.
const unbudgetedLookbackMonths = 3

// maxNamedGaps is how many missing things are named before the rest are counted.
//
// Three. A list of eleven is the ledger again with a warning colour: the reader
// skips it, and skipping it is worse than reading three they might actually go
// and fix.
const maxNamedGaps = 3

// buildMonthReady derives next month's readiness from the live store (EC-11).
//
// "Expected income" comes from the RECURRING schedule rather than from last
// month's actual income: the question is what the household has told the app to
// expect, and inferring it from history would quietly promise that an unusual
// month repeats. A household with no recurring income declared reports income as
// unknown — which is a finding, not a zero.
func buildMonthReady(app *appstate.App, now time.Time, rates currency.Rates) monthready.Input {
	next := dateutil.AddMonths(dateutil.MonthStart(now), 1)
	nextEnd := dateutil.AddMonths(next, 1)

	var in monthready.Input
	for _, r := range app.Recurring() {
		if r.Paused {
			continue // a suspended schedule is not what next month expects
		}
		conv, err := rates.Convert(r.Amount, rates.Base)
		if err != nil {
			continue
		}
		amt := conv.Amount
		if amt > 0 {
			in.ExpectedIncomeMinor += amt
			in.IncomeKnown = true
			continue
		}
		// A yearly charge landing next month is the kind that reshapes it.
		if r.Cadence == domain.CadenceYearly && !r.NextDue.IsZero() &&
			!r.NextDue.Before(next) && r.NextDue.Before(nextEnd) {
			in.AnnualCharges = append(in.AnnualCharges, monthready.Charge{Name: r.Label, Minor: amt})
		}
	}

	// Committed: what the recurring schedule and the goal plans already claim of
	// next month, from the same projection the annual grid draws.
	proj := budgetplan.Project(app.Recurring(), app.Goals(), next.Year(), int(next.Month())-1, rates.Base, rates)
	idx := int(next.Month()) - 1
	for _, amounts := range proj.Recurring {
		in.CommittedMinor += amounts[idx]
	}
	for _, amounts := range proj.GoalsByCategory {
		in.CommittedMinor += amounts[idx]
	}
	for _, amounts := range proj.GoalsByBudget {
		in.CommittedMinor += amounts[idx]
	}

	in.UnbudgetedCategories = unbudgetedCategories(app, now)
	return in
}

// unbudgetedCategories names the categories the household has been spending on
// that have no budget — the gap somebody can actually close.
func unbudgetedCategories(app *appstate.App, now time.Time) []string {
	budgeted := map[string]bool{}
	for _, b := range app.Budgets() {
		if b.CategoryID != "" {
			budgeted[b.CategoryID] = true
		}
	}
	cut := dateutil.AddMonths(dateutil.MonthStart(now), -unbudgetedLookbackMonths)
	used := map[string]bool{}
	for _, t := range app.Transactions() {
		if !t.IsExpense() || !t.CountsInReports() || t.CategoryID == "" || t.Date.Before(cut) {
			continue
		}
		if !budgeted[t.CategoryID] {
			used[t.CategoryID] = true
		}
	}
	if len(used) == 0 {
		return nil
	}
	// Ordered by the category list rather than by map iteration, so the same data
	// always produces the same sentence.
	var out []string
	for _, c := range app.Categories() {
		if used[c.ID] {
			out = append(out, c.Name)
		}
	}
	return out
}

// monthReadyLines renders next month's readiness as sentences (EC-11).
//
// Sentences, not a score: "next month: 68/100" cannot be acted on, argued with,
// or improved. Each line here names one thing somebody can go and do.
func monthReadyLines(r monthready.Readiness, base string) []string {
	var out []string
	// The headroom is a fact whatever else is missing. Gating it on everything
	// being in order meant a household with one unbudgeted category never saw the
	// figure they came for.
	switch {
	case r.Overcommitted:
		out = append(out, uistate.T("monthready.over", fmtMoney(money.New(-r.HeadroomMinor, base))))
	case r.Tight:
		out = append(out, uistate.T("monthready.tight", fmtMoney(money.New(r.HeadroomMinor, base))))
	case r.HeadroomMinor > 0:
		out = append(out, uistate.T("monthready.ready", fmtMoney(money.New(r.HeadroomMinor, base))))
	}
	for _, c := range r.Large {
		out = append(out, uistate.T("monthready.annual", c.Name, fmtMoney(money.New(c.Minor, base))))
	}
	// The trust reasons already read as noun phrases ("next month's income", "a
	// budget for Dining"), so they compose into one sentence rather than needing
	// their own copy per gap.
	// Named up to a point, then counted. Eleven categories in one sentence is the
	// ledger again with a warning colour — the reader skips it, which is worse
	// than three names they might actually go and fix.
	if reasons := r.Reasons(); len(reasons) > 0 {
		if len(reasons) > maxNamedGaps {
			extra := len(reasons) - maxNamedGaps
			out = append(out, uistate.T("monthready.missingMore",
				strings.Join(reasons[:maxNamedGaps], "; "), extra))
		} else {
			out = append(out, uistate.T("monthready.missing", strings.Join(reasons, "; ")))
		}
	}
	return out
}

// readinessNotes renders next month's readiness lines, or nothing when there is
// nothing to say (EC-11).
func readinessNotes(app *appstate.App, now time.Time, rates currency.Rates, base string) ui.Node {
	lines := monthReadyLines(monthready.Assess(buildMonthReady(app, now, rates)), base)
	if len(lines) == 0 {
		return Fragment()
	}
	rows := make([]ui.Node, 0, len(lines))
	for _, l := range lines {
		rows = append(rows, P(css.Class("muted", tw.Text13), Attr("data-testid", "month-ready-note"), l))
	}
	return Div(css.Class(tw.Mb2), Attr("data-testid", "month-ready"), rows)
}
