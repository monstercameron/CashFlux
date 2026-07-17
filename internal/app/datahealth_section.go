// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/integrity"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// dataHealthSection renders the Data tab's local integrity check (#53): the
// pure internal/integrity cross-checks run live against the dataset and any
// inconsistencies list here with plain-English explanations and drill-throughs.
func dataHealthSection() uic.Node {
	return uic.CreateElement(dataHealthCard)
}

// dataHealthChecksCount is how many distinct cross-check families run — stated
// in the healthy line so "all clear" names what was actually checked.
const dataHealthChecksCount = 9

func dataHealthCard() uic.Node {
	_ = uistate.UseDataRevision().Get()
	nav := router.UseNavigate()
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	findings := integrity.Run(integrity.Input{
		Accounts:     app.Accounts(),
		Transactions: app.Transactions(),
		Budgets:      app.Budgets(),
		Goals:        app.Goals(),
	})

	// Drill-throughs: a transaction lands on the ledger searched to its
	// description; entities land on their page.
	drill := func(f integrity.Finding) {
		switch f.EntityType {
		case "transaction":
			flt := uistate.TxFilter{Text: f.Name}.Normalize()
			uistate.PersistTxFilter(flt)
			nav.Navigate(uistate.RoutePath("/transactions"))
		case "account":
			nav.Navigate(uistate.RoutePath("/accounts"))
		case "budget":
			nav.Navigate(uistate.RoutePath("/budgets"))
		case "goal":
			nav.Navigate(uistate.RoutePath("/goals"))
		}
	}

	keyOf := func(f integrity.Finding) any { return f.ID }
	render := func(f integrity.Finding) uic.Node {
		return uic.CreateElement(dataHealthRow, dataHealthRowProps{F: f, OnDrill: drill})
	}
	return Div(Attr("data-testid", "data-health-section"),
		H4(css.Class("set-label"), uistate.T("health.sectionTitle")),
		P(css.Class("muted", tw.TextXs), uistate.T("health.sectionHint")),
		If(len(findings) == 0, P(css.Class(tw.TextFaint, tw.Text12), Attr("data-testid", "data-health-clean"),
			uistate.T("health.allClear", dataHealthChecksCount))),
		If(len(findings) > 0, Fragment(
			P(css.Class(tw.Text13), Attr("data-testid", "data-health-count"),
				uistate.T("health.findingCount", len(findings))),
			Div(css.Class("rows", tw.Mt045), MapKeyed(findings, keyOf, render)),
		)),
	)
}

// dataHealthRowProps feeds one finding row.
type dataHealthRowProps struct {
	F       integrity.Finding
	OnDrill func(integrity.Finding)
}

// dataHealthRow is its own component so the drill click hook sits at a stable
// position per row.
func dataHealthRow(p dataHealthRowProps) uic.Node {
	open := uic.UseEvent(Prevent(func() { p.OnDrill(p.F) }))
	f := p.F
	dec := currency.Decimals(f.Currency)
	amt := func(m int64) string { return money.FormatMinor(m, dec) }
	var line string
	switch f.Check {
	case integrity.CheckTransferOrphan:
		line = uistate.T("health.transferOrphan", f.Name, amt(f.AmountMinor))
	case integrity.CheckSplitSum:
		line = uistate.T("health.splitSum", f.Name, amt(f.AmountMinor), amt(f.OtherMinor))
	case integrity.CheckCurrencyMismatch:
		line = uistate.T("health.currencyMismatch", f.Name, f.Currency)
	case integrity.CheckOrphanAccount:
		line = uistate.T("health.orphanAccount", f.Name)
	case integrity.CheckLiabilitySign:
		line = uistate.T("health.liabilitySign", f.Name, amt(f.AmountMinor))
	case integrity.CheckReconcileDrift:
		line = uistate.T("health.reconcileDrift", f.Name, amt(f.OtherMinor), amt(f.AmountMinor))
	case integrity.CheckBudgetLimit:
		line = uistate.T("health.budgetLimit", f.Name)
	case integrity.CheckGoalArithmetic:
		line = uistate.T("health.goalArithmetic", f.Name)
	case integrity.CheckGoalOverfunded:
		line = uistate.T("health.goalOverfunded", f.Name, amt(f.AmountMinor), amt(f.OtherMinor))
	default:
		line = f.Name
	}
	sev := uistate.T("health.sevInfo")
	sevCls := "badge text-dim"
	if f.Severity == integrity.SevWarning {
		sev = uistate.T("health.sevWarning")
		sevCls = "badge"
	}
	return Div(css.Class("row"), Attr("data-testid", "data-health-row"),
		Style(map[string]string{"display": "flex", "justify-content": "space-between", "align-items": "center", "gap": "1rem"}),
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
			Span(ClassStr(sevCls), sev),
			Span(line)),
		Button(css.Class("btn", tw.ShrinkO), Type("button"), Attr("data-testid", "data-health-drill"),
			Title(uistate.T("health.drillTitle")), OnClick(open), uistate.T("health.drillBtn")),
	)
}
