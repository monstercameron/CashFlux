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
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// Goals is the widgetized goals surface — a thin SURFACE HOST like /budgets and
// /accounts. It builds one engine RenderCtx over the live store and renders a fixed
// set of native widget specs through the same spec/render pipeline:
//
//   - goal-summary (Native): the saved-of-target "loader" bar with Saved/Target/Left
//     inside it (hides itself when there are no goals)
//   - goal-toolbar (Native): the "Sort by" picker and the "Add goal" button
//   - goal-list    (Native): the sinking-funds card, the active GoalRow list (or the
//     first-run empty CTA), and the collapsible achieved card
func Goals() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	_ = uistate.UseDataRevision().Get()

	// GL7: on entering the goals surface, resume any goal whose pause has elapsed and
	// file a single gentle "pick it back up" nudge per goal. Clearing PausedUntil is the
	// once-guard, so this converges after one pass. Runs on mount only.
	ui.UseEffect(func() func() {
		if n, err := app.SweepEndedGoalPauses(time.Now(), func(name string) string {
			return uistate.T("goals.pauseEndedNudge", name)
		}); err == nil && n > 0 {
			uistate.BumpDataRevision()
		}
		return nil
	})

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

	// TX11: the dismissible round-up sweep card sits above the bento; the round-up
	// config flip modal renders as a sibling of the bento (outside any tile
	// transform) so its centering isn't broken.
	return Fragment(
		goalsRoundUpCard(),
		Div(css.Class("bento bento-goals"),
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
		goalsRoundUpConfigModal(),
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
}

// goalNativeSpec builds the seed spec for a Native goals tile (fixed surface).
func goalNativeSpec(id string) domain.WidgetSpec {
	return domain.WidgetSpec{SchemaVersion: domain.WidgetSpecVersion, ID: id, Kind: domain.KindNative, NativeID: id}
}
