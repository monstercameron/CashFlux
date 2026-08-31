// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/router"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// budgetRecurringWidget lists the household's active recurring commitments with
// their cadence (frequency) and per-month normalized cost, surfaced right on the
// budgets page (coworker feedback #5). It reads the confirmed recurring set
// (app.Recurring()), not raw guesses, so every row is a real repeating charge.
// Self-gates to nothing when there are no active recurrings, and the rows are
// display-only — the single footer button navigates to the full /recurring
// surface for editing (keeping per-row handlers out of the loop).
func budgetRecurringWidget(props budgetSummaryProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	nav := router.UseNavigate()
	goToRecurring := ui.UseEvent(Prevent(func() { nav.Navigate(uistate.RoutePath("/recurring")) }))
	open := ui.UseState(false)
	toggleOpen := ui.UseEvent(Prevent(func() { open.Set(!open.Get()) }))

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}

	// Active recurrings only, biggest monthly commitment first.
	recs := make([]domain.Recurring, 0)
	for _, r := range app.Recurring() {
		if r.Active() {
			recs = append(recs, r)
		}
	}
	if len(recs) == 0 {
		return Fragment() // nothing detected yet — stay quiet
	}
	absMonthly := func(r domain.Recurring) int64 {
		m := r.MonthlyEquivalent()
		if m < 0 {
			return -m
		}
		return m
	}
	sort.SliceStable(recs, func(i, j int) bool {
		return absMonthly(recs[i]) > absMonthly(recs[j])
	})

	catName := make(map[string]string, len(app.Categories()))
	for _, c := range app.Categories() {
		catName[c.ID] = c.Name
	}

	// Committed outflow, normalized to a per-month figure in the base currency. Only
	// spending recurrings (negative monthly equivalent) count toward "committed" — a
	// recurring paycheck is detected and listed, but it isn't a budget commitment, so
	// including it would understate what's actually spoken-for each month.
	var totalMonthly int64
	for _, r := range recs {
		me := r.MonthlyEquivalent()
		if me >= 0 {
			continue // income / inflow — shown in the list, but not "committed"
		}
		if conv, err := currency.ConvertBetween(-me, r.Amount.Currency, base, rates); err == nil {
			totalMonthly += conv
		}
	}

	// C609: every date is stated against a reference. The list showed "Next Jul 3,
	// 2026" beside "Next Sep 1, 2026" with nothing saying whether either had
	// happened, was about to, or belonged to the period on screen — three
	// different facts under one word.
	recurNow := time.Now()
	recurStart, recurEnd := uistate.UsePeriod().Get().Range()
	rows := make([]ui.Node, 0, len(recs))
	for _, r := range recs {
		rows = append(rows, budgetRecurringRow(r, catName, recurNow, recurStart, recurEnd))
	}

	head := Div(css.Class("brc-head"),
		Div(
			Span(css.Class("brc-total-label", tw.TextDim), uistate.T("budgets.recurring.totalLabel")),
			Span(css.Class("brc-total-val fig", tw.Fold(tw.FontDisplay)),
				uistate.T("budgets.recurring.totalVal", fmtMoney(money.New(totalMonthly, base)))),
		),
		Span(css.Class("brc-count", tw.TextDim), uistate.T("budgets.recurring.countLabel", len(recs))),
	)

	// Collapsible, like the two sections below it. This is the tallest tile on the
	// surface — thirteen rows, over a thousand pixels — for a figure that changes a
	// few times a year, so it opens closed and states its own headline: the monthly
	// commitment and how many charges make it up. That is the whole answer most
	// visits need; you open it to see WHICH charges (Cam, 2026-08-31).
	caretCls := "budget-fold-caret"
	foldAria := uistate.T("budgets.recurring.showAria")
	if open.Get() {
		caretCls += " is-open"
		foldAria = uistate.T("budgets.recurring.hideAria")
	}
	fold := Div(css.Class("budget-fold-head"),
		Button(css.Class("budget-fold-toggle"), Type("button"), Attr("data-testid", "budgets-recurring-toggle"),
			Attr("aria-expanded", ariaBool(open.Get())), Attr("aria-label", foldAria), OnClick(toggleOpen),
			Span(ClassStr(caretCls), Attr("aria-hidden", "true"),
				uiw.Icon(icon.ChevronRight, css.Class(tw.ShrinkO, tw.W4, tw.H4))),
			Span(css.Class("budget-fold-toggle-label"), uistate.T("budgets.recurring.title")),
			Span(css.Class("budget-fold-toggle-hint"),
				uistate.TN("budgets.recurring.foldHintOne", "budgets.recurring.foldHintMany",
					len(recs), fmtMoney(money.New(totalMonthly, base)))),
		),
	)

	var body ui.Node = Fragment()
	if open.Get() {
		body = Fragment(
			head,
			P(css.Class("muted", tw.Text13), uistate.T("budgets.recurring.desc")),
			Div(css.Class("brc-rows"), rows),
			Div(css.Class("brc-foot"),
				Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "budgets-recurring-manage"),
					OnClick(goToRecurring),
					uiw.Icon(icon.Repeat, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
					Span(uistate.T("budgets.recurring.manage"))),
			),
		)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "budget-recurring", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: Div(css.Class("brc budget-fold"), Attr("data-testid", "budgets-recurring"), fold, body),
	})
}

// budgetRecurringRow renders one recurring commitment: its label, category, the
// cadence as a frequency pill, next-due date, amount, and — when the cadence
// isn't monthly — the normalized per-month figure so charges compare fairly.
// Display-only (no handlers), safe to build inside the row loop.
func budgetRecurringRow(r domain.Recurring, catName map[string]string, now, periodStart, periodEnd time.Time) ui.Node {
	cat := catName[r.CategoryID]
	if cat == "" {
		cat = uistate.T("budgets.recurring.uncategorized")
	}
	meta := []ui.Node{Span(cat)}
	if r.Autopay {
		meta = append(meta, Span(css.Class(tw.TextFaint), " · "+uistate.T("budgets.recurring.autopay")))
	}
	// C609: the date carries its own meaning — overdue, due inside the period on
	// screen, or the schedule's next date after it.
	if state := budgeting.ClassifyRecurDate(r.NextDue, now, periodStart, periodEnd); state != budgeting.RecurUnscheduled {
		when := uistate.LoadPrefs().FormatDate(r.NextDue)
		meta = append(meta, Span(ClassStr(recurDateClass(state)), Attr("data-testid", "brc-date-"+string(state)),
			" · "+uistate.T(recurDateKey(state), when)))
	}

	// Amount block: the charge as-is, plus a per-month equivalent when the cadence
	// isn't already monthly (so a $1,200 annual bill reads "≈ $100/mo" too).
	amountNodes := []ui.Node{Span(css.Class("brc-amt fig", tw.Fold(tw.FontDisplay)), fmtMoney(r.Amount))}
	if r.Cadence != domain.CadenceMonthly {
		amountNodes = append(amountNodes, Span(css.Class("brc-permo fig", tw.TextFaint),
			uistate.T("budgets.recurring.perMonth", fmtMoney(money.New(r.MonthlyEquivalent(), r.Amount.Currency)))))
	}

	return Div(css.Class("brc-row"), Attr("data-testid", "budgets-recurring-row"),
		Span(css.Class("brc-cadence"), recurCadence(r.Cadence)),
		Div(css.Class("brc-body"),
			Span(css.Class("brc-label", tw.Fold(tw.FontDisplay)), r.Label),
			Div(css.Class("brc-meta", tw.Text12, tw.TextDim), meta),
		),
		Div(css.Class("brc-amtcol"), amountNodes),
	)
}

// recurDateKey / recurDateClass give each date state its own words and tone
// (C609). Only "overdue" is a call to action, so only it is toned; the other two
// are context and stay quiet.
func recurDateKey(state budgeting.RecurDateState) string {
	switch state {
	case budgeting.RecurOverdue:
		return "budgets.recurring.overdue"
	case budgeting.RecurAfterPeriod:
		return "budgets.recurring.afterPeriod"
	case budgeting.RecurBeforePeriod:
		return "budgets.recurring.beforePeriod"
	}
	return "budgets.recurring.dueInPeriod"
}

func recurDateClass(state budgeting.RecurDateState) string {
	if state == budgeting.RecurOverdue {
		return "brc-date is-overdue"
	}
	return "brc-date " + tw.Fold(tw.TextFaint)
}
