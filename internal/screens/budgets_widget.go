// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetrender"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// Budgets is the widgetized budgets surface. Like /accounts and /transactions, the
// page is a thin SURFACE HOST: it builds one engine RenderCtx over the live store and
// renders a fixed set of widget specs through the same spec/render pipeline the
// dashboard uses (safeRenderSpec). Every visible block is its own engine tile —
//
//   - budget-summary (Native): the spend/budgeted/left stat grid, income/methodology
//     assign banner, sinking-fund set-aside, and the over/near alert banner + badges
//   - budget-toolbar (Native): the methodology picker, 50/30/20 template, "Add budget",
//     a Formulas reveal toggle, and the smart-insights action
//   - budget-list    (Native): the health-sorted budget rows (BudgetRow), or the
//     first-run empty-state CTA
//   - budget-formula (Native): the opt-in "Budget metrics" FormulaBuilder (revealed by
//     the toolbar toggle) — ties budget custom fields + the formula engine together
//
// The tiles share the same computed picture (computeBudgetView) and the Formulas
// toggle atom; the host just decides which specs are present and the engine renders
// each through its per-widget error boundary.
func Budgets() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}

	// Re-render on any data mutation (a budget CRUD, a transaction added elsewhere, a
	// method switch).
	_ = uistate.UseDataRevision().Get()

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

	// The placement set. Summary, toolbar, and list are always present; summary hides
	// itself (renders nothing) when there are no budgets. The formula tile appears only
	// when the toolbar's Formulas toggle is on.
	specs := []domain.WidgetSpec{
		// "Needs attention" opens the page on real work: the three most over/at-risk
		// budgets, above settings and methodology (self-gates to nothing when healthy).
		budgetNativeSpec("budget-attention"),
		budgetNativeSpec("budget-summary"),
		budgetNativeSpec("budget-toolbar"),
		budgetNativeSpec("budget-list"),
		// BG9: the annual plan-vs-actual grid as its own full-width cell below the list
		// (self-gates to nothing when there are no budgets).
		budgetNativeSpec("budget-annualgrid"),
		// Self-gating: renders nothing unless the method is zero-based (savings/
		// investment goals counted toward the assigned total).
		budgetNativeSpec("budget-savings"),
	}
	// BG2: the flex methodology replaces the per-budget list with one pooled flex
	// meter plus the fixed/non-monthly composition. Keep the toolbar (it holds the
	// method picker to switch back) but swap the budget-heavy tiles for the flex view.
	if budgeting.ParseMethodology(app.Settings().BudgetMethodology) == budgeting.MethodFlex {
		specs = []domain.WidgetSpec{
			budgetNativeSpec("budget-toolbar"),
			budgetNativeSpec("budget-flex"),
		}
	}

	// XC6: the dismissible month-close sweep card sits above the bento; the sweep
	// config flip modal renders as a sibling of the bento (outside any tile
	// transform) so its centering isn't broken.
	return Fragment(
		budgetsSweepCard(),
		Div(css.Class("bento bento-budgets"),
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
		),
		budgetsSweepConfigModal(),
		budgetsCoverAllModal(),
		budgetsMonthCloseModal(),
		flexAssignSheet(),
	)
}

// init registers the budgets-surface widget bodies with the engine render registry,
// keyed by NativeID. The bodies read the live store (c.App) and the shared atoms,
// never surface locals.
func init() {
	R := widgetrender.Register

	R("budget-summary", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(budgetSummaryWidget, budgetSummaryProps{App: c.App})
	})
	R("budget-toolbar", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(budgetToolbarWidget, budgetToolbarProps{App: c.App})
	})
	R("budget-list", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(budgetListWidget, budgetListProps{App: c.App})
	})
	R("budget-annualgrid", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(budgetAnnualGridWidget, budgetListProps{App: c.App})
	})
	R("budget-savings", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(budgetSavingsWidget, budgetSummaryProps{App: c.App})
	})
	R("budget-flex", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(budgetFlexWidget, budgetSummaryProps{App: c.App})
	})
}

// budgetNativeSpec builds the seed spec for a Native budgets tile. The surface is
// fixed (not user-reconfigurable or persisted), so the spec is constructed inline
// rather than catalogued in widgetregistry.
func budgetNativeSpec(id string) domain.WidgetSpec {
	return domain.WidgetSpec{SchemaVersion: domain.WidgetSpecVersion, ID: id, Kind: domain.KindNative, NativeID: id}
}
