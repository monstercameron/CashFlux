// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"sort"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
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
	sort.SliceStable(recs, func(i, j int) bool {
		return recs[i].MonthlyEquivalent() > recs[j].MonthlyEquivalent()
	})

	catName := make(map[string]string, len(app.Categories()))
	for _, c := range app.Categories() {
		catName[c.ID] = c.Name
	}

	// Total committed, normalized to a per-month figure in the base currency.
	var totalMonthly int64
	for _, r := range recs {
		if conv, err := currency.ConvertBetween(r.MonthlyEquivalent(), r.Amount.Currency, base, rates); err == nil {
			totalMonthly += conv
		}
	}

	rows := make([]ui.Node, 0, len(recs))
	for _, r := range recs {
		rows = append(rows, budgetRecurringRow(r, catName))
	}

	head := Div(css.Class("brc-head"),
		Div(
			Span(css.Class("brc-total-label", tw.TextDim), uistate.T("budgets.recurring.totalLabel")),
			Span(css.Class("brc-total-val fig", tw.Fold(tw.FontDisplay)),
				uistate.T("budgets.recurring.totalVal", fmtMoney(money.New(totalMonthly, base)))),
		),
		Span(css.Class("brc-count", tw.TextDim), uistate.T("budgets.recurring.countLabel", len(recs))),
	)

	section := uiw.EntityListSection(uiw.EntityListSectionProps{
		Title:  uistate.T("budgets.recurring.title"),
		TestID: "budgets-recurring",
		Body: Fragment(
			head,
			P(css.Class("muted", tw.Text13), uistate.T("budgets.recurring.desc")),
			Div(css.Class("brc-rows"), rows),
			Div(css.Class("brc-foot"),
				Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "budgets-recurring-manage"),
					OnClick(goToRecurring),
					uiw.Icon(icon.Repeat, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
					Span(uistate.T("budgets.recurring.manage"))),
			),
		),
	})
	return uiw.Widget(uiw.WidgetProps{
		ID: "budget-recurring", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: section,
	})
}

// budgetRecurringRow renders one recurring commitment: its label, category, the
// cadence as a frequency pill, next-due date, amount, and — when the cadence
// isn't monthly — the normalized per-month figure so charges compare fairly.
// Display-only (no handlers), safe to build inside the row loop.
func budgetRecurringRow(r domain.Recurring, catName map[string]string) ui.Node {
	cat := catName[r.CategoryID]
	if cat == "" {
		cat = uistate.T("budgets.recurring.uncategorized")
	}
	meta := []ui.Node{Span(cat)}
	if r.Autopay {
		meta = append(meta, Span(css.Class(tw.TextFaint), " · "+uistate.T("budgets.recurring.autopay")))
	}
	if !r.NextDue.IsZero() {
		meta = append(meta, Span(css.Class(tw.TextFaint),
			" · "+uistate.T("budgets.recurring.nextDue", uistate.LoadPrefs().FormatDate(r.NextDue))))
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
