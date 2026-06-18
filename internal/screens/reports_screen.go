//go:build js && wasm

package screens

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Reports is the read-only reporting screen (B21): for the period chosen in the
// top bar it shows income / expense / net, a plain-English summary, and spending
// by category compared to the prior period — all from the pure internal/reports
// core, so the figures match the rest of the app. Charts come in a follow-up.
func Reports() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), uistate.T("common.notReady")))
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	txns := app.Transactions()

	// The viewed period is the shared top-bar window; the comparison is the
	// immediately preceding window of the same length.
	w := uistate.UsePeriod().Get()
	cs, ce := w.Range()
	ps, pe := w.Shift(-1).Range()

	flow, _ := reports.IncomeVsExpense(txns, cs, ce, rates)
	rows, _ := reports.SpendingByCategory(txns, cs, ce, true, ps, pe, rates)

	cats := app.Categories()
	catName := make(map[string]string, len(cats))
	for _, c := range cats {
		catName[c.ID] = c.Name
	}
	nameOf := func(id string) string {
		if n := catName[id]; n != "" {
			return n
		}
		return uistate.T("reports.uncategorized")
	}
	fmtMinor := func(v int64) string { return fmtMoney(money.New(v, base)) }
	narrative := reports.SpendingNarrative(rows, true, fmtMinor, func(id string) string { return catName[id] })

	// Category rows are plain text (no interactive controls), so building them in
	// a loop is safe (no On* hooks involved).
	var rowNodes []ui.Node
	for _, r := range rows {
		if r.Amount == 0 && r.Prior == 0 {
			continue
		}
		delta := Fragment()
		if r.HasDelta && r.Amount != r.Prior {
			// Spending up is red (worse), down is green (better).
			tone, arrow := "text-down", "▲"
			if r.DeltaPct < 0 {
				tone, arrow = "text-up", "▼"
			}
			pct := r.DeltaPct
			if pct < 0 {
				pct = -pct
			}
			delta = Span(Class("row-meta "+tone), fmt.Sprintf("%s %d%%", arrow, pct))
		}
		rowNodes = append(rowNodes, Div(Class("row"),
			Div(Class("row-main"), Span(Class("row-desc"), nameOf(r.CategoryID))),
			delta,
			Span(Class("budget-amount"), fmtMinor(r.Amount)),
		))
	}

	var catBody ui.Node
	if len(rowNodes) == 0 {
		catBody = P(Class("empty"), uistate.T("reports.empty"))
	} else {
		catBody = Div(Class("rows"), rowNodes)
	}

	net := money.New(flow.Net(), base)
	return Div(
		Div(Class("stat-grid"),
			stat(uistate.T("dashboard.income"), fmtMoney(money.New(flow.Income, base)), "pos"),
			stat(uistate.T("dashboard.spending"), fmtMoney(money.New(flow.Expense, base)), "neg"),
			stat(uistate.T("reports.net"), fmtMoney(net), accentFor(net)),
			stat(uistate.T("dashboard.savingsRate"), fmt.Sprintf("%d%%", flow.SavingsRate()), ""),
		),
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("reports.byCategory")),
			P(Class("muted"), narrative),
			catBody,
		),
	)
}
