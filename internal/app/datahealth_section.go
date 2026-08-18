// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/integrity"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/router"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
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

	// LF-8: the unfinished-work counts, as distinct from the integrity findings
	// above. An uncategorized transaction is not an ERROR — it is a chore — and
	// mixing the two would either bury real errors under a pile of housekeeping or
	// dress housekeeping up as corruption. Separate line, separate framing.
	//
	// The staleness window comes from the app's own freshness config rather than a
	// default, so this count and the accounts page cannot disagree about the same
	// account.
	hygiene := integrity.Hygiene(integrity.HygieneInput{
		Accounts:     app.Accounts(),
		Transactions: app.Transactions(),
		Windows:      app.FreshnessWindows(),
		Now:          time.Now(),
	})
	goHygiene := uic.UseEvent(func(e uic.Event) {
		route := e.JSValue().Get("currentTarget").Call("getAttribute", "data-route").String()
		if route != "" && route != "<null>" {
			nav.Navigate(uistate.RoutePath(route))
		}
	})
	hygieneChips := make([]any, 0, len(hygiene))
	for _, h := range hygiene {
		if h.N == 0 {
			// "0 uncategorized" is noise; a panel listing what you have already
			// finished is a panel about itself.
			continue
		}
		hygieneChips = append(hygieneChips, Button(css.Class("btn-link", tw.Text12),
			Type("button"), Attr("data-testid", "data-hygiene-"+h.Kind),
			Attr("data-route", h.Route), OnClick(goHygiene),
			uistate.T("health.hygiene."+h.Kind, h.N)))
	}
	var hygieneLine uic.Node = Fragment()
	if len(hygieneChips) > 0 {
		hygieneLine = Div(css.Class("data-hygiene"), Attr("data-testid", "data-hygiene"),
			Span(css.Class(tw.TextFaint, tw.Text12), uistate.T("health.hygieneLead")),
			Fragment(hygieneChips...))
	}

	keyOf := func(f integrity.Finding) any { return f.ID }
	render := func(f integrity.Finding) uic.Node {
		return uic.CreateElement(dataHealthRow, dataHealthRowProps{F: f, OnDrill: drill})
	}
	return Div(Attr("data-testid", "data-health-section"),
		H4(css.Class("set-label"), uistate.T("health.sectionTitle")),
		P(css.Class("muted", tw.TextXs), uistate.T("health.sectionHint")),
		hygieneLine,
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
	case integrity.CheckTransferLegsDisagree:
		line = uistate.T("health.transferLegsDisagree", f.Name, amt(f.AmountMinor))
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
