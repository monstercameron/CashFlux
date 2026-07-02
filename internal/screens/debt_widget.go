// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetrender"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// DebtPlanner is the widgetized /debt surface — a thin SURFACE HOST like /accounts and
// /goals. It builds one engine RenderCtx over the live store and renders a fixed set of
// native debt tiles through the same spec/render pipeline. Every derived figure the tiles
// show comes from the engine variable surface (engineenv) or is banded by the persisted
// DebtConfig — the page has no inline thresholds or hand-rolled totals:
//
//   - debt-summary  (Native): total owed + debt-free date + engine ratio chips
//   - debt-toolbar  (Native): the Debt-metrics Formulas toggle, Manage accounts, Add debt
//   - debt-list     (Native): the payoff-ladder DebtRow cards (or the no-debts empty state)
//   - debt-strategy (Native): the shared snowball-vs-avalanche planner
//   - debt-credit   (Native): credit-card health (only when a card exists)
//   - debt-loans    (Native): installment loans (only when a loan exists)
//   - debt-payoff   (Native): the manual single-debt payoff what-if
//   - debt-formula  (Native): the opt-in Debt-metrics FormulaBuilder (toolbar toggle)
func DebtPlanner() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	_ = uistate.UseDataRevision().Get()
	formulasAtom := uistate.UseDebtShowFormulas()

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	accounts := app.Accounts()
	txns := app.Transactions()
	rctx := widgetrender.RenderCtx{
		App: app, Accounts: accounts, Txns: txns,
		ScopedAccounts: accounts, ScopedTxns: txns,
		Rates: rates, Base: base,
		Start: time.Time{}, End: time.Now(),
	}

	// Which optional panels to place: mirror the old page's conditional credit/loans
	// sections, driven by what liability types actually exist.
	var hasCC, hasLoans, hasDebts bool
	for _, ac := range accounts {
		if ac.Archived || ac.Class != domain.ClassLiability {
			continue
		}
		hasDebts = true
		if ac.Type == domain.TypeCreditCard {
			hasCC = true
		}
		if isInstallmentLoan(ac.Type) {
			hasLoans = true
		}
	}

	specs := []domain.WidgetSpec{
		debtNativeSpec("debt-summary"),
		debtNativeSpec("debt-toolbar"),
		debtNativeSpec("debt-list"),
	}
	if hasDebts {
		specs = append(specs, debtNativeSpec("debt-strategy"))
		if hasCC {
			specs = append(specs, debtNativeSpec("debt-credit"))
		}
		if hasLoans {
			specs = append(specs, debtNativeSpec("debt-loans"))
		}
		specs = append(specs, debtNativeSpec("debt-payoff"))
	}
	if formulasAtom.Get() {
		specs = append(specs, debtNativeSpec("debt-formula"))
	}

	return Div(css.Class("bento bento-debt"),
		MapKeyed(specs,
			func(sp domain.WidgetSpec) any { return sp.ID },
			func(sp domain.WidgetSpec) ui.Node {
				c := rctx
				c.Spec = sp
				if node, ok := safeRenderSpec(sp, c); ok {
					return node
				}
				return Fragment()
			},
		),
	)
}

// init registers the debt-surface widget bodies with the engine render registry.
func init() {
	R := widgetrender.Register
	R("debt-summary", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(debtSummaryWidget, debtSummaryProps{App: c.App})
	})
	R("debt-toolbar", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(debtToolbarWidget, debtToolbarProps{App: c.App})
	})
	R("debt-list", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(debtListWidget, debtListProps{App: c.App})
	})
	R("debt-strategy", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(debtStrategyWidget, debtPanelProps{App: c.App})
	})
	R("debt-credit", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(debtCreditWidget, debtPanelProps{App: c.App})
	})
	R("debt-loans", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(debtLoansWidget, debtPanelProps{App: c.App})
	})
	R("debt-payoff", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(debtPayoffWidget, debtPanelProps{App: c.App})
	})
	R("debt-formula", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(debtFormulaWidget, debtPanelProps{App: c.App})
	})
}

// debtNativeSpec builds the seed spec for a Native debt tile (fixed surface).
func debtNativeSpec(id string) domain.WidgetSpec {
	return domain.WidgetSpec{SchemaVersion: domain.WidgetSpecVersion, ID: id, Kind: domain.KindNative, NativeID: id}
}
