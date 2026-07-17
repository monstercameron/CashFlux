// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

// Number provenance for the Annual Review masthead (#56): each headline figure
// is clickable and opens a small popover naming the plain facts behind it —
// how many transactions were counted across how many accounts, the window,
// and what was deliberately left out (transfers, exclude-from-reports rows).
// The facts come from the pure internal/provenance package, which mirrors the
// exact counting rules of ledger.PeriodTotals (the function the figures come
// from), so the popover can never disagree with the number it explains.

import (
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// rptaProvFigProps drives one masthead figure with a provenance popover.
type rptaProvFigProps struct {
	ID      string   // anchor wrap id, unique per figure (also seeds testids)
	Label   string   // the figure caption (INCOME, SPENDING, …)
	Value   string   // the formatted headline number
	Tone    string   // value tone class suffix ("", "up", "down")
	Sub     string   // optional sub-line under the value
	SubTone string   // optional tone class suffix for the sub-line
	Title   string   // popover heading
	Lines   []string // the plain facts behind the number, in display order
	Extra   ui.Node  // optional trailing fig content (e.g. the net-worth sparkline)
}

// rptaProvFig renders a masthead figure whose value is a button opening the
// provenance popover. Own component so its hooks (open state, toggle, popover
// anchor/dismiss effects) sit at a stable call-site per figure.
func rptaProvFig(p rptaProvFigProps) ui.Node {
	open := ui.UseState(false)
	toggle := ui.UseEvent(Prevent(func() { open.Set(!open.Get()) }))
	uiw.DismissPopover(open.Get(), p.ID, func() { open.Set(false) })
	uiw.AnchorPopover(open.Get(), p.ID)

	vCls := "rpta-fig-v " + tw.Fold(tw.FontDisplay)
	if p.Tone != "" {
		vCls += " rpta-tone-" + p.Tone
	}
	subCls := "rpta-fig-sub rpta-muted"
	if p.SubTone != "" {
		subCls = "rpta-fig-sub rpta-tone-" + p.SubTone
	}
	menuCls := "add-menu rpta-prov-pop"
	if !open.Get() {
		menuCls += " hidden-menu"
	}
	var lines []ui.Node
	for _, l := range p.Lines {
		lines = append(lines, Div(css.Class("rpta-prov-line"), l))
	}
	var extra ui.Node = Fragment()
	if p.Extra != nil {
		extra = p.Extra
	}
	return Div(css.Class("rpta-fig", "add-wrap"), Attr("id", p.ID), Attr("data-testid", p.ID),
		Span(css.Class("rpta-fig-k"), p.Label),
		Button(ClassStr("rpta-fig-btn "+vCls), Type("button"),
			Attr("data-testid", p.ID+"-btn"),
			Attr("aria-haspopup", "dialog"), Attr("aria-expanded", boolStr(open.Get())),
			Title(uistate.T("rpta.provHint")),
			OnClick(toggle),
			p.Value),
		If(p.Sub != "", Span(ClassStr(subCls), p.Sub)),
		extra,
		Div(ClassStr(menuCls), Attr("role", "dialog"),
			Attr("aria-label", p.Title), Attr("data-testid", p.ID+"-pop"),
			Div(css.Class("rpta-prov-title"), p.Title),
			lines,
		),
	)
}
