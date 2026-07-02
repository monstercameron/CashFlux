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

// NotificationCenter is the widgetized notifications surface — a thin SURFACE HOST like
// /todo and /goals. It renders a fixed set of native tiles through the shared
// spec/render pipeline:
//
//   - notif-summary (Native): alert count + severity breakdown + "N new since last visit"
//     (also marks everything read on open); hides itself when the feed is empty
//   - notif-toolbar (Native): the severity filter strip + Clear all
//   - notif-list    (Native): the severity-sorted feed of notifyRow cards (⋯ per item)
func NotificationCenter() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
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

	specs := []domain.WidgetSpec{
		notifNativeSpec("notif-summary"),
		notifNativeSpec("notif-toolbar"),
		notifNativeSpec("notif-list"),
	}

	return Div(css.Class("bento bento-notif"),
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

// init registers the notifications-surface widget bodies with the engine render registry.
func init() {
	R := widgetrender.Register
	R("notif-summary", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(notifSummaryWidget, notifProps{App: c.App})
	})
	R("notif-toolbar", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(notifToolbarWidget, notifProps{App: c.App})
	})
	R("notif-list", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(notifListWidget, notifProps{App: c.App})
	})
}

// notifNativeSpec builds the seed spec for a Native notifications tile (fixed surface).
func notifNativeSpec(id string) domain.WidgetSpec {
	return domain.WidgetSpec{SchemaVersion: domain.WidgetSpecVersion, ID: id, Kind: domain.KindNative, NativeID: id}
}
