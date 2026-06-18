//go:build js && wasm

package screens

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/subscriptions"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Subscriptions lists recurring charges detected from transaction history (B25):
// each subscription's cadence, charge, normalized monthly cost, and next renewal,
// plus the total monthly/annual burden. Read-only over the pure detection core.
func Subscriptions() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), uistate.T("common.notReady")))
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	pr := uistate.UsePrefs().Get()

	subs, _ := subscriptions.Detect(app.Transactions(), rates, 2)

	var annual int64
	for _, s := range subs {
		annual += s.AnnualAmount()
	}

	// Rows are plain text (no interactive controls), so building them in a loop is
	// safe (no On* hooks involved).
	var rowNodes []ui.Node
	for _, s := range subs {
		meta := subscriptionCadenceLabel(s.Cadence) + " · " + uistate.T("subs.next", pr.FormatDate(s.NextRenewal))
		rowNodes = append(rowNodes, Div(Class("row"),
			Div(Class("row-main"),
				Span(Class("row-desc"), s.Name),
				Span(Class("row-meta"), meta),
			),
			Span(Class("row-meta"), uistate.T("subs.perMonth", fmtMoney(money.New(s.MonthlyAmount(), base)))),
			Span(Class("budget-amount"), fmtMoney(money.New(s.Amount, base))),
		))
	}

	var body ui.Node
	if len(rowNodes) == 0 {
		body = P(Class("empty"), uistate.T("subs.empty"))
	} else {
		body = Div(Class("rows"), rowNodes)
	}

	return Div(
		If(len(subs) > 0, Div(Class("stat-grid"),
			stat(uistate.T("subs.monthlyBurden"), fmtMoney(money.New(subscriptions.MonthlyTotal(subs), base)), "neg"),
			stat(uistate.T("subs.annualBurden"), fmtMoney(money.New(annual, base)), ""),
			stat(uistate.T("subs.count"), fmt.Sprintf("%d", len(subs)), ""),
		)),
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("nav.subscriptions")),
			body,
		),
	)
}

// subscriptionCadenceLabel renders a detected cadence as a friendly label.
func subscriptionCadenceLabel(c subscriptions.Cadence) string {
	switch c {
	case subscriptions.CadenceWeekly:
		return uistate.T("subs.weekly")
	case subscriptions.CadenceYearly:
		return uistate.T("subs.yearly")
	default:
		return uistate.T("subs.monthly")
	}
}
