// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// budgets_income_alloc.go answers the question the /budgets hero never answered:
// how much of your income have you actually budgeted?
//
// The hero band answers "how much of my BUDGET have I spent" — spent, budgeted,
// left. That is one question, and it is not the one you have just been asked
// after configuring "Budget income". Cam set an income basis, saved, and every
// figure on the page stayed where it was, because the basis fed the band's
// denominator only under the zero-based method and he was on Simple (the
// default). Worse, on the sample household the hero reads "Budgeted $9,582.50"
// beside an income of $5,900 — the plan exceeds earnings by $3,682.50 and the
// headline says nothing at all.
//
// The bar itself is not new. A well-built allocation bar already existed for the
// zero-based hero — income split into expenses, savings and the unassigned gap,
// with a tick where income runs out — but its only caller (budgetAssignBanner)
// was orphaned when the B1 hero consolidation landed, so nothing rendered it.
// This revives it as a method-agnostic component and gives it the caption that
// states the relationship in words.

// allocRead is the resolved allocation picture: the figures, the segment widths,
// and the words that describe them. Split out from rendering so the arithmetic
// and the state machine are testable without a DOM.
type allocRead struct {
	Pool       int64 // income (+ rolled-over leftover) the plan is measured against
	Budgeted   int64
	Savings    int64
	Unassigned int64 // negative when the plan runs past the pool
	PlanPct    int   // assigned as a percent of the pool; may exceed 100
	State      string

	BudgetedPct int // bar: fill up to the income tick
	OverPct     int // bar: striped fill past the income tick
	SavingsPct  int
	GapPct      int
	MarkerPct   int // -1 when there is nothing past the tick to mark

	// Spending, measured on the SAME money axis as the segments above. Zero-based
	// has three quantities that nest — income holds the plan, the plan holds the
	// spending — and nesting cannot be read off two charts with different
	// denominators. Plotting spend against this scale lets the rail be compared to
	// the plan and the income tick by LENGTH, with no percentages to reconcile.
	Spent    int64
	SpentPct int
}

// resolveAllocation computes the allocation read for a budget view.
func resolveAllocation(v budgetView) allocRead {
	r := allocRead{
		Pool:      v.BannerIncome + v.RolledOver,
		Budgeted:  v.TotalLimit,
		Savings:   v.SavingsAssigned,
		MarkerPct: -1,
	}
	if r.Pool <= 0 {
		return r
	}
	assigned := r.Budgeted + r.Savings
	r.Unassigned = budgeting.ToAssign(r.Pool, assigned)
	r.PlanPct = int(assigned * 100 / r.Pool)
	switch {
	case assigned == 0:
		r.State = "empty" // an income basis with no budgets yet — day one
	case r.Unassigned < 0:
		r.State = "over"
	case r.Unassigned == 0:
		r.State = "exact"
	default:
		r.State = "under"
	}

	// Widths are a share of the LARGER of the pool or what is assigned, so an
	// over-assigned plan still fits the track and the overshoot stays visible
	// rather than being clipped at 100%.
	scale := r.Pool
	if assigned > scale {
		scale = assigned
	}
	gap := r.Unassigned
	if gap < 0 {
		gap = 0
	}
	pctOf := func(n int64) int {
		if scale <= 0 || n <= 0 {
			return 0
		}
		if p := int(n * 100 / scale); p < 100 {
			return p
		}
		return 100
	}
	r.Spent = v.TotalSpent
	r.SpentPct = pctOf(r.Spent)
	r.BudgetedPct = pctOf(r.Budgeted)
	r.GapPct = pctOf(gap)
	r.SavingsPct = 100 - r.BudgetedPct - r.GapPct // the remainder, so the bar fills exactly
	if r.SavingsPct < 0 {
		r.SavingsPct = 0
	}
	// The income tick only means something when the plan runs past it. When the
	// plan is under, the END of the track already IS the pool, so a tick at 100%
	// would mark a boundary the track edge already draws.
	if r.State == "over" {
		r.MarkerPct = int(r.Pool * 100 / scale)
		// Everything past the tick is money that does not exist, and it must not
		// wear the healthy accent. The first draft did, and a bar that is entirely
		// green while its caption says "more than you earn" argues with itself —
		// the reader believes the colour.
		if r.BudgetedPct > r.MarkerPct {
			r.OverPct = r.BudgetedPct - r.MarkerPct
			r.BudgetedPct = r.MarkerPct
		}
	}
	return r
}

// budgetIncomeAllocation renders the income allocation read: a caption naming
// what is being compared, the allocation bar, and — only when it carries
// something the caption did not — a legend.
//
// It renders nothing when there is no income figure to compare against, so a
// household that has not set a basis (or has no income history) sees exactly
// what it saw before, plus the "Budget income" call to action.
//
// `hist` is true when the viewed period has already closed, which switches every
// tense: "more than you earn" is a claim about an ongoing state, and a month that
// ended cannot be in one.
//
// `action` is the change-the-basis control, passed in rather than built here
// because its OnClick registers a hook, and hooks belong to the component that
// owns them.
//
// `showSpend` adds the spend rail and its caption, folding "how much of the plan
// have I spent" onto this bar's axis. It is set only where the caller has
// dropped the separate loader band (zero-based live views); elsewhere the band
// still owns that question and a rail here would draw it twice.
func budgetIncomeAllocation(v budgetView, action ui.Node, hist bool, showSpend bool) ui.Node {
	r := resolveAllocation(v)
	if r.Pool <= 0 {
		return Fragment()
	}
	base := v.Base

	// The denominator is labelled for what it actually is. Once last month's
	// leftover rolls in, the figure is no longer income, and calling it income
	// puts a wrong number under a confident label.
	denomKey := "budgets.allocOfIncome"
	if v.RolledOver > 0 {
		denomKey = "budgets.allocOfAvailable"
	}

	// Day one: an income basis is set and no budgets exist yet. This is literally
	// the next thing a household sees after configuring income for the first time,
	// so it reads as an invitation rather than a 0% failure report.
	if r.State == "empty" {
		return Div(css.Class("budget-alloc"), Attr("data-testid", "budgets-income-alloc"),
			Attr("data-state", "empty"),
			Div(css.Class("budget-alloc-cap"),
				Span(css.Class("budget-alloc-cap-label"), uistate.T(allocCapKey(hist))),
				Span(css.Class("budget-alloc-relation"), Attr("data-testid", "budgets-alloc-relation"),
					uistate.T("budgets.allocNoBudgets", fmtMoney(money.New(r.Pool, base)))),
				Span(css.Class(tw.MlAuto), action),
			),
		)
	}

	relation, relationCls := allocRelation(r, base, hist)
	aria := uistate.T("budgets.allocAria", r.PlanPct)
	if r.State == "over" {
		aria = uistate.T("budgets.allocAriaOver", r.PlanPct)
	}

	return Div(css.Class("budget-alloc"), Attr("data-testid", "budgets-income-alloc"),
		Attr("data-state", r.State),
		Div(css.Class("budget-alloc-cap"),
			Span(css.Class("budget-alloc-cap-label"), uistate.T(allocCapKey(hist))),
			Span(css.Class(tw.MlAuto), action),
		),
		// The figure and its denominator are ADJACENT. Splitting them across the
		// caption's width is what made "162%" read as "162% of what?".
		Div(css.Class("budget-alloc-line"),
			Span(css.Class("budget-alloc-pct fig"), Attr("data-testid", "budgets-alloc-pct"),
				uistate.T("budgets.allocPct", r.PlanPct)),
			Span(css.Class("budget-alloc-denom"), Attr("data-testid", "budgets-alloc-denom"),
				uistate.T(denomKey, fmtMoney(money.New(r.Pool, base)))),
			Span(ClassStr(relationCls), Attr("data-testid", "budgets-alloc-relation"), relation),
		),
		// Bar and marker share a non-clipping wrapper so the income tick can
		// protrude past the bar, which clips its own segments.
		Div(css.Class("zbb-alloc-wrap"),
			Div(css.Class("zbb-alloc budget-alloc-bar"), Attr("role", "img"), Attr("aria-label", aria),
				If(r.BudgetedPct > 0, Div(css.Class("zbb-alloc-seg is-exp"), Attr("style", fmt.Sprintf("width:%d%%", r.BudgetedPct)))),
				If(r.SavingsPct > 0, Div(css.Class("zbb-alloc-seg is-sav"), Attr("style", fmt.Sprintf("width:%d%%", r.SavingsPct)))),
				If(r.OverPct > 0, Div(css.Class("zbb-alloc-seg is-overflow"), Attr("data-testid", "budgets-alloc-overflow"),
					Attr("style", fmt.Sprintf("width:%d%%", r.OverPct)))),
				If(r.GapPct > 0, Div(css.Class("zbb-alloc-seg is-gap"), Attr("style", fmt.Sprintf("width:%d%%", r.GapPct)))),
			),
			If(r.MarkerPct >= 0, Div(css.Class("zbb-alloc-marker"), Attr("style", fmt.Sprintf("left:%d%%", r.MarkerPct)),
				Attr("data-testid", "budgets-alloc-marker"), Attr("title", uistate.T("budgets.allocMarker")))),
			// The spend rail shares this wrapper — and therefore this axis — with the
			// plan above it. Read down the same x: where the rail stops against the
			// income tick is what you have actually spent of what you actually have;
			// where the PLAN stops against that tick is whether the plan was ever
			// affordable. Two facts, one axis, no percentages to reconcile.
			If(showSpend, Div(css.Class("budget-zbb-rail"), Attr("data-testid", "budgets-spend-rail"),
				Attr("role", "img"), Attr("aria-label", uistate.T("budgets.spendRailAria", r.SpentPct)),
				Div(css.Class("budget-zbb-rail-fill"), Attr("style", fmt.Sprintf("width:%d%%", r.SpentPct))),
			)),
		),
		// Replaces the three figures the loader band used to carry. The band plotted
		// spending on its OWN axis, which made it a second chart of the same money —
		// this states the pair in words instead, under the picture that shows it.
		If(showSpend, Div(css.Class("budget-zbb-note"), Attr("data-testid", "budgets-spend-cap"),
			spendCaption(r, base))),
		// The legend only earns its place when it says something the line above
		// did not. With no savings there are two segments and both are already
		// named in words directly above, so it would be pure restatement.
		// The legend appears whenever savings is part of the picture — which in
		// zero-based is ALWAYS, not merely once a target has been set. Gating it on
		// Savings > 0 meant the concept did not exist for anyone who had not already
		// found it: no segment, no legend, and the only trace a collapsed tile at the
		// very bottom of the surface. A stated empty slot is information; an omitted
		// one is not (Cam, 2026-08-31).
		If(r.Savings > 0 || v.Method == budgeting.MethodZeroBased, Div(css.Class("zbb-legend"),
			zbbLegendItem("is-exp", uistate.T("budgets.allocBudgets"), r.Budgeted, base),
			allocSavingsLegend(r, base),
			allocThirdLegend(r, base),
		)),
	)
}

// allocCapKey picks the eyebrow label's tense.
func allocCapKey(hist bool) string {
	if hist {
		return "budgets.allocCapHist"
	}
	return "budgets.allocCap"
}

// allocRelation renders the plan-versus-income relationship and the class that
// tones it, in the tense the viewed period calls for.
func allocRelation(r allocRead, base string, hist bool) (string, string) {
	cls := "budget-alloc-relation"
	switch r.State {
	case "over":
		key := "budgets.allocOver"
		if hist {
			key = "budgets.allocOverHist"
		}
		return uistate.T(key, fmtMoney(money.New(-r.Unassigned, base))), cls + " is-over"
	case "exact":
		key := "budgets.allocExact"
		if hist {
			key = "budgets.allocExactHist"
		}
		return uistate.T(key), cls
	}
	key := "budgets.allocUnder"
	if hist {
		key = "budgets.allocUnderHist"
	}
	return uistate.T(key, fmtMoney(money.New(r.Unassigned, base))), cls
}

// allocThirdLegend is the legend's last slot: what is still unbudgeted, or how
// far past income the plan runs. It never shows a misleading "$0.00 not yet
// budgeted" beside an over-income caption.
func allocThirdLegend(r allocRead, base string) ui.Node {
	if r.State == "over" {
		return zbbLegendItemTone("is-over", "zbb-legend-val-over", uistate.T("budgets.allocOverShort"), -r.Unassigned, base)
	}
	gap := r.Unassigned
	if gap < 0 {
		gap = 0
	}
	return zbbLegendItemTone("is-gap", "", uistate.T("budgets.allocUnassigned"), gap, base)
}

// allocSavingsLegend renders the savings slot, including when nothing is assigned
// to it yet — the empty state reads "Savings targets — none set" rather than the
// row simply not being there.
func allocSavingsLegend(r allocRead, base string) ui.Node {
	if r.Savings > 0 {
		return zbbLegendItem("is-sav", uistate.T("budgets.zbbSavings"), r.Savings, base)
	}
	// Same element shape as zbbLegendItemTone, so the empty slot lines up with its
	// siblings instead of sitting a few pixels off in its own layout.
	return Div(css.Class("zbb-legend-item"), Attr("data-testid", "budgets-alloc-savings-none"),
		Span(css.Class("zbb-legend-dot is-sav")),
		Span(css.Class("zbb-legend-label"), uistate.T("budgets.zbbSavings")),
		Span(css.Class("zbb-legend-val", tw.TextFaint), uistate.T("budgets.allocSavingsNone")))
}

// spendCaption states what was spent against what was assigned, and names the
// savings share when there is one. A single "assigned" figure hides which part of
// it is a ceiling to stay under and which part is a floor to reach.
func spendCaption(r allocRead, base string) string {
	assigned := fmtMoney(money.New(r.Budgeted+r.Savings, base))
	spent := fmtMoney(money.New(r.Spent, base))
	if r.Savings > 0 {
		return uistate.T("budgets.spendOfAssignedSplit", spent, assigned,
			fmtMoney(money.New(r.Savings, base)))
	}
	return uistate.T("budgets.spendOfAssigned", spent, assigned)
}
