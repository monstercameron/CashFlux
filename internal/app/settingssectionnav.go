// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// settingsNavKeys are the section headings of the global settings panel, in
// document order. Each maps 1:1 to an `H4.set-label` rendered by the two settings
// columns; the nav jumps to them by matching the heading text, so it needs no
// edits to (and no ids injected into) the column components themselves.
var settingsNavKeys = []string{
	"settings.householdMembers",
	"settings.screens",
	"settings.baseCurrency",
	"settings.budgetMethod",
	"settings.exchangeRates",
	"settings.freshnessTitle",
	"settings.notifyTitle",
	"settings.appearance",
	"settings.preferences",
	"settings.aiTitle",
	"settings.backendTitle",
	"settings.data",
	"applock.section",
	"settings.languages",
}

type settingsNavBtnProps struct{ Label string }

// settingsNavBtn is one jump-link in the settings section nav. Its own component
// so the click hook stays at a stable position (the framework forbids On* inside a
// variable-length loop). Clicking scrolls the matching section heading into view.
func settingsNavBtn(p settingsNavBtnProps) uic.Node {
	onClick := uic.UseEvent(func() {
		doc := js.Global().Get("document")
		nodes := doc.Call("querySelectorAll", ".set-label")
		for i := 0; i < nodes.Get("length").Int(); i++ {
			el := nodes.Call("item", i)
			if strings.TrimSpace(el.Get("textContent").String()) == p.Label {
				el.Call("scrollIntoView", map[string]any{"behavior": "smooth", "block": "start"})
				return
			}
		}
	})
	return Button(css.Class("btn", tw.Text12), Type("button"), Attr("title", p.Label), OnClick(onClick), p.Label)
}

// settingsSectionNav renders a wrapping row of jump-links across the top of the
// dense global settings panel (§6.12 / 24403), so a section can be reached in one
// click instead of scrolling a long two-column form. Purely additive: it locates
// each section by its heading text at click time, leaving the column components
// untouched.
func settingsSectionNav() uic.Node {
	args := []any{
		css.Class("set-section-nav", tw.Flex, tw.FlexWrap, tw.ItemsCenter, tw.Gap15, tw.Mb3, tw.Pb2, tw.BorderB, tw.BorderLine),
		Span(css.Class(tw.Text12, tw.TextFaint), uistate.T("settings.jumpTo")),
	}
	for _, k := range settingsNavKeys {
		args = append(args, uic.CreateElement(settingsNavBtn, settingsNavBtnProps{Label: uistate.T(k)}))
	}
	return Div(args...)
}
