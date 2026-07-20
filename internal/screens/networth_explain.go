// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// The "?" explainer for the two signature graphics.
//
// Both graphics are unusual shapes — a waterfall and a gap chart — and a reader
// meeting one for the first time should not have to infer how to read it. Each
// therefore carries a "?" beside its section title that opens a short,
// plain-language description of what the picture is telling them: what is drawn
// where, and what to conclude when it changes. It explains the PICTURE, never
// the algorithm.
//
// It deliberately reuses the app's existing number-provenance convention (the
// Annual Review masthead figures, reports_provenance.go): the same
// `add-wrap` / `add-menu` / `hidden-menu` anchoring, the same DismissPopover +
// AnchorPopover pair that keeps a popover on screen and closes it on an outside
// click or Escape, the same `role="dialog"` + `aria-haspopup` / `aria-expanded`
// wiring, and the same `<id>` / `<id>-btn` / `<id>-pop` testid shape.

// nwsExplainProps configures one graphic's explainer.
type nwsExplainProps struct {
	// ID seeds the anchor wrap id and every testid, so it must be unique per
	// explainer on the page.
	ID string
	// Title heads the popover; Lines are the plain-language sentences, in
	// reading order.
	Title string
	Lines []string
}

// nwsExplain renders the "?" button and its popover. Own component so the hooks
// (open state, toggle, dismiss + anchor effects) sit at a stable call-site per
// graphic rather than inside a section builder.
func nwsExplain(p nwsExplainProps) ui.Node {
	open := ui.UseState(false)
	toggle := ui.UseEvent(Prevent(func() { open.Set(!open.Get()) }))
	uiw.DismissPopover(open.Get(), p.ID, func() { open.Set(false) })
	uiw.AnchorPopover(open.Get(), p.ID)

	menuCls := "add-menu nws-explain-pop"
	if !open.Get() {
		menuCls += " hidden-menu"
	}
	lines := make([]ui.Node, 0, len(p.Lines))
	for _, l := range p.Lines {
		lines = append(lines, P(css.Class("nws-explain-line"), l))
	}
	// The accessible name carries the graphic's own title, so several "?"
	// buttons on one page read as distinct actions rather than as "button"
	// repeated.
	label := uistate.T("nws.explainAria", p.Title)
	return Span(css.Class("nws-explain", "add-wrap"), Attr("id", p.ID),
		Button(css.Class("nws-explain-btn"), Type("button"),
			Attr("data-testid", p.ID+"-btn"),
			Attr("aria-haspopup", "dialog"), Attr("aria-expanded", boolStr(open.Get())),
			Attr("aria-label", label), Title(label),
			OnClick(toggle),
			Span(Attr("aria-hidden", "true"), "?")),
		Div(ClassStr(menuCls), Attr("role", "dialog"),
			Attr("aria-label", p.Title), Attr("data-testid", p.ID+"-pop"),
			Div(css.Class("nws-explain-title"), p.Title),
			lines,
		),
	)
}

// nwsBridgeExplain is the "?" for THE BRIDGE.
func nwsBridgeExplain() ui.Node {
	return ui.CreateElement(nwsExplain, nwsExplainProps{
		ID:    "nws-explain-bridge",
		Title: uistate.T("nws.bridgeTitle"),
		Lines: []string{
			uistate.T("nws.explainBridge1"),
			uistate.T("nws.explainBridge2"),
			uistate.T("nws.explainBridge3"),
		},
	})
}

// nwsSidesExplain is the "?" for TWO SIDES.
func nwsSidesExplain() ui.Node {
	return ui.CreateElement(nwsExplain, nwsExplainProps{
		ID:    "nws-explain-sides",
		Title: uistate.T("nws.sidesTitle"),
		Lines: []string{
			uistate.T("nws.explainSides1"),
			uistate.T("nws.explainSides2"),
			uistate.T("nws.explainSides3"),
		},
	})
}
