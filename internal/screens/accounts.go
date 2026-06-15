//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Accounts lists assets and liabilities with live balances and a net-worth
// summary, reading from the app store.
func Accounts() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), "App state is not ready yet."))
	}

	accounts := app.Accounts()
	txns := app.Transactions()
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	net, assets, liabilities, _ := ledger.NetWorth(accounts, txns, rates)

	var assetRows, liabRows []ui.Node
	for _, ac := range accounts {
		if ac.Archived {
			continue
		}
		bal, _ := ledger.Balance(ac, txns)
		row := accountRow(ac, bal)
		if ac.Class == domain.ClassLiability {
			liabRows = append(liabRows, row)
		} else {
			assetRows = append(assetRows, row)
		}
	}

	return Div(
		Div(Class("stat-grid"),
			stat("Net worth", fmtMoney(net), accentFor(net)),
			stat("Assets", fmtMoney(assets), "pos"),
			stat("Liabilities", fmtMoney(liabilities), "neg"),
		),
		Section(Class("card"),
			H2(Class("card-title"), "Assets"),
			IfElse(len(assetRows) == 0,
				P(Class("empty"), "No asset accounts yet."),
				Div(Class("rows"), assetRows),
			),
		),
		Section(Class("card"),
			H2(Class("card-title"), "Liabilities"),
			IfElse(len(liabRows) == 0,
				P(Class("empty"), "No liabilities — nice."),
				Div(Class("rows"), liabRows),
			),
		),
	)
}

func accountRow(ac domain.Account, bal money.Money) ui.Node {
	return Div(Class("row"),
		Div(Class("row-main"),
			Span(Class("row-desc"), ac.Name),
			Span(Class("row-meta"), humanizeType(string(ac.Type))+" · "+ac.Currency),
		),
		Span(Class(amountClass(bal)), fmtMoney(bal)),
	)
}
