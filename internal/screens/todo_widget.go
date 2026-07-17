// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"syscall/js"
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

// Todo is the widgetized to-do surface — a thin SURFACE HOST like /budgets, /goals and
// /accounts. It renders a fixed set of native widget specs through the shared
// spec/render pipeline:
//
//   - todo-summary (Native): a done-of-total completion "loader" bar with Open / Overdue
//     / Done figures inside it (hides itself when there are no tasks)
//   - todo-toolbar (Native): the priority filter, the hide/show-done toggle, and Add task
//   - todo-list    (Native): the parent/child task tree (or the first-run empty CTA)
func Todo() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	_ = uistate.UseDataRevision().Get()

	// QA CF-28: an assistant "[Open it](/todo#taskID)" link (or any hash deep
	// link) lands with the fragment in the URL — consume it once on mount into
	// the deep-link flash so the target task is scrolled to and pulsed, instead
	// of appearing unfocused somewhere in the sorted list.
	ui.UseEffect(func() func() {
		if hash := js.Global().Get("location").Get("hash").String(); len(hash) > 1 {
			uistate.SetDeepLinkFocus(`[id="` + strings.TrimPrefix(hash, "#") + `"]`)
		}
		return nil
	}, "todo-hash-focus")

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
		todoNativeSpec("todo-summary"),
		todoNativeSpec("todo-toolbar"),
		todoNativeSpec("todo-list"),
	}

	return Div(css.Class("bento bento-todo"),
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

// init registers the to-do-surface widget bodies with the engine render registry.
func init() {
	R := widgetrender.Register
	R("todo-summary", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(todoSummaryWidget, todoSummaryProps{App: c.App})
	})
	R("todo-toolbar", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(todoToolbarWidget, todoToolbarProps{App: c.App})
	})
	R("todo-list", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(todoListWidget, todoListProps{App: c.App})
	})
}

// todoNativeSpec builds the seed spec for a Native to-do tile (fixed surface).
func todoNativeSpec(id string) domain.WidgetSpec {
	return domain.WidgetSpec{SchemaVersion: domain.WidgetSpecVersion, ID: id, Kind: domain.KindNative, NativeID: id}
}
