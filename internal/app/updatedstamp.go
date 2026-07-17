// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/icon"
	ui "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// UpdatedStamp is the top-bar "Updated 4m ago" chip — the freshness leg of the
// persistent scope strip (parity scan: "Everyone · All accounts · Jul 2026 ·
// Updated 4 min ago"). Local-first, the honest analog of a bank product's
// "last synced" is the last change to the dataset: the newest audit entry.
// Clicking opens /activity, where that change (and its Undo) lives. Renders
// nothing before the first recorded change.
func UpdatedStamp() uic.Node {
	_ = uistate.UseDataRevision().Get() // re-render the moment the data changes
	nav := router.UseNavigate()
	openActivity := uic.UseEvent(func() { nav.Navigate(uistate.RoutePath("/activity")) })

	// A minute-ticker keeps the relative age honest while the app sits idle —
	// without it "now" would read "now" an hour later. The tick state only
	// changes the rendered label, never the data.
	tick := uic.UseState(0)
	uic.UseEffect(func() func() {
		cb := js.FuncOf(func(js.Value, []js.Value) any {
			tick.Set(tick.Get() + 1)
			return nil
		})
		id := js.Global().Call("setInterval", cb, 60_000)
		return func() {
			js.Global().Call("clearInterval", id)
			cb.Release()
		}
	}, "updated-stamp-tick")

	recent := auditview.Feed.Recent(1)
	if len(recent) == 0 {
		return Fragment()
	}
	age := freshness.RelAge(recent[0].At, time.Now().UTC())
	if age == "" {
		return Fragment()
	}
	label := uistate.T("topbar.updated", age)
	if age == "now" {
		label = uistate.T("topbar.updatedNow")
	}
	return Button(css.Class("tb-updated", tw.InlineFlex, tw.ItemsCenter, tw.Gap1, tw.TextDim),
		Type("button"),
		Attr("data-testid", "topbar-updated"),
		Attr("title", uistate.T("topbar.updatedTitle", recent[0].Summary)),
		Attr("aria-label", uistate.T("topbar.updatedTitle", recent[0].Summary)),
		OnClick(openActivity),
		ui.Icon(icon.History, css.Class(tw.W35, tw.H35)),
		// The label collapses to icon-only on crowded widths (CSS) — the title/
		// aria-label carry the full sentence either way.
		Span(css.Class("tb-updated-label"), label),
	)
}
