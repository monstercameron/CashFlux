// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/attribution"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// THE BRIDGE — signature graphic #1.
//
// A waterfall that decomposes the window's net-worth movement: where you
// started, what money you kept, what the market did, what debt you paid off,
// what debt you took on, what got revalued, and where you ended. It exists
// because a net-worth line that rises identically whether you saved the money
// or whether a condo was re-appraised is not an answer, and those are two
// completely different stories about the same household.
//
// The residual is ALWAYS drawn, even at zero, in a deliberately different
// visual language (outlined and dashed, never a filled leg): it is the part the
// named legs could not explain, and a waterfall that quietly absorbs it into a
// neighbouring bar is a picture, not arithmetic.
//
// Geometry: the SVG's X axis is measured in COLUMNS — column i spans x = i…i+1
// — and the SVG stretches freely (preserveAspectRatio="none"). That is what
// lets the HTML label grid beneath line up with the bars exactly at any width
// with no measurement and no JavaScript. The bars carry no text, so the
// stretching costs nothing; every word on this graphic is real HTML, which is
// also what makes it readable by assistive tech and selectable by the user.

// nwsBridgeCol is one column of the waterfall: an anchor (start / end) or a leg.
type nwsBridgeCol struct {
	Kind attribution.LegKind
	// Anchor marks the start and end state columns, which are drawn from the
	// baseline rather than floating.
	Anchor bool
	Label  string
	// From / To are the running net-worth values this column spans.
	From, To int64
	// AmountMinor is what the column reports: the state for an anchor, the
	// signed movement for a leg.
	AmountMinor int64
}

// nwsBridgeCols lays the bridge out left to right. Zero-valued named legs are
// dropped (a bar of nothing teaches nothing); the residual is kept regardless.
func nwsBridgeCols(b attribution.Bridge) []nwsBridgeCol {
	cols := []nwsBridgeCol{{
		Anchor: true, Label: uistate.T("nws.legStart"),
		From: b.StartMinor, To: b.StartMinor, AmountMinor: b.StartMinor,
	}}
	running := b.StartMinor
	for _, k := range attribution.BridgeLegOrder {
		amt := b.Leg(k)
		if amt == 0 && k != attribution.LegResidual {
			continue
		}
		cols = append(cols, nwsBridgeCol{
			Kind: k, Label: nwsLegLabel(k),
			From: running, To: running + amt, AmountMinor: amt,
		})
		running += amt
	}
	cols = append(cols, nwsBridgeCol{
		Anchor: true, Label: uistate.T("nws.legEnd"),
		From: b.EndMinor, To: b.EndMinor, AmountMinor: b.EndMinor,
	})
	return cols
}

// nwsLegLabel names one leg in the household's own language.
func nwsLegLabel(k attribution.LegKind) string {
	switch k {
	case attribution.LegMoneyKept:
		return uistate.T("nws.legMoneyKept")
	case attribution.LegMarketMovement:
		return uistate.T("nws.legMarket")
	case attribution.LegDebtPaidDown:
		return uistate.T("nws.legDebtPaid")
	case attribution.LegNewDebt:
		return uistate.T("nws.legNewDebt")
	case attribution.LegRevaluation:
		return uistate.T("nws.legRevaluation")
	}
	return uistate.T("nws.legResidual")
}

// nwsLegExplain is the one-line "what counts here" the Detail view prints beside
// each leg, so the reader never has to guess what a bar contains.
func nwsLegExplain(k attribution.LegKind) string {
	switch k {
	case attribution.LegMoneyKept:
		return uistate.T("nws.legMoneyKeptWhat")
	case attribution.LegMarketMovement:
		return uistate.T("nws.legMarketWhat")
	case attribution.LegDebtPaidDown:
		return uistate.T("nws.legDebtPaidWhat")
	case attribution.LegNewDebt:
		return uistate.T("nws.legNewDebtWhat")
	case attribution.LegRevaluation:
		return uistate.T("nws.legRevaluationWhat")
	}
	return uistate.T("nws.legResidualWhat")
}

// nwsBridgeScale is the vertical mapping. The baseline is the LOWEST running
// value rather than zero: a household whose net worth is $350k and moved $4k
// would otherwise get six invisible legs under one tall bar. The anchors are
// drawn hollow and the section states its floor in words, so a truncated axis is
// disclosed rather than smuggled.
type nwsBridgeScale struct{ lo, hi float64 }

func newNwsBridgeScale(cols []nwsBridgeCol) nwsBridgeScale {
	lo, hi := float64(cols[0].From), float64(cols[0].From)
	for _, c := range cols {
		for _, v := range []float64{float64(c.From), float64(c.To)} {
			if v < lo {
				lo = v
			}
			if v > hi {
				hi = v
			}
		}
	}
	pad := (hi - lo) * 0.18
	if pad <= 0 {
		pad = absFloat(hi)*0.1 + 1
	}
	return nwsBridgeScale{lo: lo - pad, hi: hi + pad}
}

// y maps a value to the 0..100 viewBox, top-down.
func (s nwsBridgeScale) y(v int64) float64 {
	span := s.hi - s.lo
	if span <= 0 {
		return 50
	}
	return 100 * (s.hi - float64(v)) / span
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// nwsBridge renders the waterfall: the SVG bars, the aligned HTML label grid
// that doubles as the graphic's text equivalent, and the stacked fallback that
// takes over at narrow pane widths (same legs, same figures, read downward
// instead of across — the graphic changes FORM rather than clipping).
func nwsBridge(v nwsView) ui.Node {
	b := v.Bridge
	cols := nwsBridgeCols(b)
	scale := newNwsBridgeScale(cols)
	n := len(cols)

	// The smallest bar height that still reads as a bar. A leg too small to see
	// at this scale is still drawn — its figure beneath is the authority.
	const minBar = 0.8

	kids := []any{
		css.Class("nws-bridge-svg"),
		Attr("viewBox", fmt.Sprintf("0 0 %d 100", n)),
		Attr("preserveAspectRatio", "none"),
		Attr("role", "img"),
		Attr("data-testid", "nws-bridge-svg"),
		Attr("aria-label", uistate.T("nws.bridgeAria",
			fmtMoney(money.New(b.StartMinor, v.Base)),
			fmtMoney(money.New(b.EndMinor, v.Base)))),
	}

	// Dashed connectors first, so the bars sit on top of them.
	for i := 0; i < n-1; i++ {
		y := scale.y(cols[i].To)
		kids = append(kids, Line(css.Class("nws-bridge-connect"),
			Attr("vector-effect", "non-scaling-stroke"),
			Attr("x1", fmt.Sprintf("%.3f", float64(i)+0.16)),
			Attr("x2", fmt.Sprintf("%.3f", float64(i)+1.84)),
			Attr("y1", fmt.Sprintf("%.3f", y)),
			Attr("y2", fmt.Sprintf("%.3f", y)),
		))
	}

	for i, c := range cols {
		x := float64(i) + 0.16
		var top, height float64
		cls := "nws-bar-up"
		switch {
		case c.Anchor:
			top = scale.y(c.To)
			height = 100 - top
			cls = "nws-bar-anchor"
		default:
			yFrom, yTo := scale.y(c.From), scale.y(c.To)
			top, height = yFrom, yTo-yFrom
			if height < 0 {
				top, height = yTo, -height
			}
			if height < minBar {
				height = minBar
			}
			switch {
			case c.Kind == attribution.LegResidual:
				cls = "nws-bar-residual"
			case c.AmountMinor < 0:
				cls = "nws-bar-down"
			}
		}
		kids = append(kids, Rect(css.Class(cls),
			Attr("vector-effect", "non-scaling-stroke"),
			Attr("x", fmt.Sprintf("%.3f", x)),
			Attr("width", "0.68"),
			Attr("y", fmt.Sprintf("%.3f", top)),
			Attr("height", fmt.Sprintf("%.3f", height)),
		))
	}

	// The aligned label grid: one column per bar, in the same order. This is the
	// graphic's text equivalent — real, selectable, screen-readable HTML.
	labelKids := []any{
		css.Class("nws-bridge-labels"),
		Attr("data-testid", "nws-bridge-labels"),
		Style(map[string]string{"grid-template-columns": fmt.Sprintf("repeat(%d, minmax(0, 1fr))", n)}),
	}
	for _, c := range cols {
		labelKids = append(labelKids, Div(css.Class("nws-bridge-label"),
			Span(css.Class("nws-bridge-name"), c.Label),
			Span(ClassStr("nws-bridge-amount"+nwsColAmountCls(c)),
				Attr("data-testid", "nws-bridge-amount"),
				Attr("data-leg", string(c.Kind)),
				nwsColAmountText(c, v.Base)),
		))
	}

	// The narrow-pane form: the same legs as rows.
	stackKids := []any{css.Class("nws-bridge-stack"), Attr("data-testid", "nws-bridge-stack")}
	var widest int64 = 1
	for _, c := range cols {
		if !c.Anchor && absMinor(c.AmountMinor) > widest {
			widest = absMinor(c.AmountMinor)
		}
	}
	for _, c := range cols {
		rowCls := "nws-bridge-srow"
		if c.Anchor {
			rowCls += " is-anchor"
		}
		var bar ui.Node = Fragment()
		if !c.Anchor {
			pct := absMinor(c.AmountMinor) * 100 / widest
			barCls := "nws-bridge-sbar is-up"
			switch {
			case c.Kind == attribution.LegResidual:
				barCls = "nws-bridge-sbar is-residual"
			case c.AmountMinor < 0:
				barCls = "nws-bridge-sbar is-down"
			}
			bar = Div(ClassStr(barCls), Style(map[string]string{"width": fmt.Sprintf("%d%%", pct)}))
		}
		stackKids = append(stackKids, Div(ClassStr(rowCls),
			Div(Span(c.Label), bar),
			Span(ClassStr("nws-bridge-amount"+nwsColAmountCls(c)), nwsColAmountText(c, v.Base)),
		))
	}

	return Div(css.Class("nws-bridge"),
		Svg(kids...),
		Div(labelKids...),
		Div(stackKids...),
		// The truncated axis is disclosed, not hidden: the bars are drawn against
		// this floor, not against zero.
		P(css.Class("nws-sec-note"), Style(map[string]string{"margin": "0.4rem 0 0"}),
			Attr("data-testid", "nws-bridge-floor"),
			uistate.T("nws.bridgeFloor", fmtMoney(money.New(int64(scale.lo), v.Base)))),
	)
}

// nwsColAmountCls tones a column's figure: gains take the accent, the residual
// stays deliberately neutral, and an anchor is a state rather than a movement.
func nwsColAmountCls(c nwsBridgeCol) string {
	switch {
	case c.Anchor:
		return " is-anchor"
	case c.Kind == attribution.LegResidual:
		return " is-residual"
	case c.AmountMinor >= 0:
		return " is-up"
	}
	return ""
}

// nwsColAmountText prints a state plainly and a movement with its sign, so the
// waterfall can be read as arithmetic straight off the labels.
func nwsColAmountText(c nwsBridgeCol, base string) string {
	if c.Anchor {
		return fmtMoney(money.New(c.AmountMinor, base))
	}
	sign := "+"
	if c.AmountMinor < 0 {
		sign = "−"
	}
	return sign + fmtMoney(money.New(absMinor(c.AmountMinor), base))
}
