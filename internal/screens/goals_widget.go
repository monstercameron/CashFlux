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

// Goals is the widgetized goals surface — a thin SURFACE HOST like /budgets and
// /accounts. It builds one engine RenderCtx over the live store and renders a fixed
// set of native widget specs through the same spec/render pipeline:
//
//   - goal-summary (Native): the saved-of-target "loader" bar with Saved/Target/Left
//     inside it (hides itself when there are no goals)
//   - goal-toolbar (Native): the smart action, a "Goal metrics" Formulas toggle, and
//     the "Add goal" button
//   - goal-list    (Native): the sinking-funds card, the active GoalRow list (or the
//     first-run empty CTA), and the collapsible achieved card
//   - goal-formula (Native): the opt-in "Goal metrics" FormulaBuilder (toolbar toggle)
func Goals() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	_ = uistate.UseDataRevision().Get()
	formulasAtom := uistate.UseGoalsShowFormulas()

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

	specs := []domain.WidgetSpec{
		goalNativeSpec("goal-summary"),
		goalNativeSpec("goal-toolbar"),
		goalNativeSpec("goal-list"),
	}
	if formulasAtom.Get() {
		specs = append(specs, goalNativeSpec("goal-formula"))
	}

	return Div(css.Class("bento bento-goals"),
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

// init registers the goals-surface widget bodies with the engine render registry.
func init() {
	R := widgetrender.Register
	R("goal-summary", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(goalSummaryWidget, goalSummaryProps{App: c.App})
	})
	R("goal-toolbar", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(goalToolbarWidget, goalToolbarProps{App: c.App})
	})
	R("goal-list", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(goalListWidget, goalListProps{App: c.App})
	})
	R("goal-formula", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(goalFormulaWidget, goalFormulaProps{App: c.App})
	})
}

// goalNativeSpec builds the seed spec for a Native goals tile (fixed surface).
func goalNativeSpec(id string) domain.WidgetSpec {
	return domain.WidgetSpec{SchemaVersion: domain.WidgetSpecVersion, ID: id, Kind: domain.KindNative, NativeID: id}
}
