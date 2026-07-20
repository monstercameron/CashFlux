// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/cashflow"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/runway"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// tideline SVG geometry (unitless viewBox; the element scales to its column).
const (
	tideW     = 1000.0
	tideH     = 190.0
	tidePadX  = 10.0
	tideMidY  = 62.0 // baseline for the amount ticks
	tideTickH = 44.0 // max tick length up/down
	tideCushT = 126.0
	tideCushB = 176.0
	tideBaseY = 179.0 // area fill floor
)

// rhyTideline renders the signature hero band: a pay-cycle SVG with income
// up-ticks (accent) and outflow down-ticks (muted) scaled by amount, a running
// cushion line beneath, a today marker, and the pinch flag. Pure — built from
// the runway.PayCycle plus the dated events that make the ticks. Ticks carry an
// SVG <title> tooltip (name · amount · date). Posted-vs-scheduled is not
// modelled at this layer (every projected occurrence is forward-looking), so
// ticks render solid; the .is-scheduled variant is reserved for when settlement
// state reaches the projection.
func rhyTideline(pc runway.PayCycle, events []cashflow.Event, base string, dec int, from time.Time) ui.Node {
	if len(events) == 0 && len(pc.Cushion) == 0 {
		return rhyTideEmpty(pc.HasIncome)
	}

	xOf := func(day int) float64 {
		span := float64(pc.WindowDays)
		if span < 1 {
			span = 1
		}
		f := float64(day) / span
		if f < 0 {
			f = 0
		}
		if f > 1 {
			f = 1
		}
		return tidePadX + f*(tideW-2*tidePadX)
	}

	// Amount scale for the ticks.
	var maxAbs int64 = 1
	for _, e := range events {
		a := e.Amount
		if a < 0 {
			a = -a
		}
		if a > maxAbs {
			maxAbs = a
		}
	}

	kids := []any{
		css.Class("rhy-tide-svg"), Attr("viewBox", fmt.Sprintf("0 0 %.0f %.0f", tideW, tideH)),
		Attr("preserveAspectRatio", "none"), Attr("width", "100%"), Attr("height", "190"),
		Attr("role", "img"), Attr("data-testid", "rhy-tideline"),
		Attr("aria-label", uistate.T("rhythm.tideAria")),
		// Axis baseline through the tick lane.
		Line(css.Class("rhy-axis"), Attr("x1", fmt.Sprintf("%.1f", tidePadX)), Attr("y1", fmt.Sprintf("%.1f", tideMidY)),
			Attr("x2", fmt.Sprintf("%.1f", tideW-tidePadX)), Attr("y2", fmt.Sprintf("%.1f", tideMidY))),
	}

	// Cushion area + line beneath the ticks.
	if len(pc.Cushion) > 0 {
		lo, hi := pc.Cushion[0].Balance, pc.Cushion[0].Balance
		for _, d := range pc.Cushion {
			if d.Balance < lo {
				lo = d.Balance
			}
			if d.Balance > hi {
				hi = d.Balance
			}
		}
		if hi == lo {
			hi = lo + 1
		}
		yOf := func(bal int64) float64 {
			f := float64(bal-lo) / float64(hi-lo)
			return tideCushB - f*(tideCushB-tideCushT)
		}
		var pts strings.Builder
		for _, d := range pc.Cushion {
			fmt.Fprintf(&pts, "%.1f,%.1f ", xOf(d.Day), yOf(d.Balance))
		}
		line := strings.TrimSpace(pts.String())
		// Area path: down to the floor and back.
		first := pc.Cushion[0]
		last := pc.Cushion[len(pc.Cushion)-1]
		area := fmt.Sprintf("M %.1f,%.1f L %s L %.1f,%.1f Z",
			xOf(first.Day), tideBaseY, strings.ReplaceAll(line, " ", " L "), xOf(last.Day), tideBaseY)
		kids = append(kids,
			Path(css.Class("rhy-cushion-area"), Attr("d", area)),
			Polyline(css.Class("rhy-cushion"), Attr("points", line), Attr("fill", "none")),
		)
	}

	// Amount ticks (income up, outflow down), each with a hover tooltip.
	for _, e := range events {
		mag := e.Amount
		if mag < 0 {
			mag = -mag
		}
		h := float64(mag) / float64(maxAbs) * tideTickH
		if h < 3 {
			h = 3
		}
		x := xOf(e.Day)
		cls := "rhy-tick-out"
		y2 := tideMidY + h
		if e.Amount > 0 {
			cls = "rhy-tick-in"
			y2 = tideMidY - h
		}
		when := startOfDayLocal(from).AddDate(0, 0, e.Day)
		tip := fmt.Sprintf("%s · %s · %s", e.Label, fmtMoney(money.New(e.Amount, base)), when.Format("Mon Jan 2"))
		kids = append(kids, G(
			Title(tip),
			Line(css.Class(cls), Attr("x1", fmt.Sprintf("%.1f", x)), Attr("y1", fmt.Sprintf("%.1f", tideMidY)),
				Attr("x2", fmt.Sprintf("%.1f", x)), Attr("y2", fmt.Sprintf("%.1f", y2))),
		))
	}

	// Today marker.
	kids = append(kids, Line(css.Class("rhy-today"),
		Attr("x1", fmt.Sprintf("%.1f", xOf(0))), Attr("y1", "8"),
		Attr("x2", fmt.Sprintf("%.1f", xOf(0))), Attr("y2", fmt.Sprintf("%.1f", tideBaseY))))

	// Pinch dot on the cushion line.
	if len(pc.Cushion) > 0 {
		lo, hi := pc.Cushion[0].Balance, pc.Cushion[0].Balance
		for _, d := range pc.Cushion {
			if d.Balance < lo {
				lo = d.Balance
			}
			if d.Balance > hi {
				hi = d.Balance
			}
		}
		if hi == lo {
			hi = lo + 1
		}
		py := tideCushB - float64(pc.Pinch.AmountMinor-lo)/float64(hi-lo)*(tideCushB-tideCushT)
		dotCls := "rhy-pinch-dot"
		if pc.Pinch.Negative {
			dotCls += " is-neg"
		}
		kids = append(kids, Circle(css.Class(dotCls),
			Attr("cx", fmt.Sprintf("%.1f", xOf(pc.Pinch.Day))), Attr("cy", fmt.Sprintf("%.1f", py)), Attr("r", "4.5")))
	}

	return Svg(kids...)
}

// rhyPinchNote renders the pinch caption below the band: amber "tightest"
// normally, red only when the projected cushion goes negative.
func rhyPinchNote(pc runway.PayCycle, base string) ui.Node {
	if !pc.HasIncome && len(pc.Cushion) == 0 {
		return Fragment()
	}
	amt := fmtMoney(money.New(pc.Pinch.AmountMinor, base))
	when := pc.Pinch.Date.Format("Jan 2")
	if pc.Pinch.Negative {
		short := fmtMoney(money.New(-pc.Pinch.AmountMinor, base))
		return P(css.Class("rhy-pinch-note is-neg"), Attr("data-testid", "rhy-pinch"),
			uistate.T("rhythm.pinchNeg", when, short))
	}
	return P(css.Class("rhy-pinch-note"), Attr("data-testid", "rhy-pinch"),
		uistate.T("rhythm.pinch", when, amt))
}

// rhyTideEmpty is the graceful no-data band: a prompt to add income (the driver
// of the pay cycle) or, once there are outflows but no income, the same nudge.
func rhyTideEmpty(hasIncome bool) ui.Node {
	msg := uistate.T("rhythm.tideNoIncome")
	if !hasIncome {
		msg = uistate.T("rhythm.tideTiny")
	}
	return Div(css.Class("rhy-tide-empty"), Attr("data-testid", "rhy-tideline-empty"),
		P(css.Class("muted"), msg))
}

// startOfDayLocal truncates to local midnight (mirrors runway's projection start).
func startOfDayLocal(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
