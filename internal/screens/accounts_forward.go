// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/accountflow"
	"github.com/monstercameron/CashFlux/internal/acctproject"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// forwardHorizonDays is how far the account-detail projection looks. Ninety days
// is roughly a quarter: far enough to cross several pay cycles and the quarterly
// bills, near enough that "today's recurring set continues" is still a
// defensible assumption rather than a guess dressed as a forecast.
const forwardHorizonDays = 90

// accountForwardLine renders the next 90 days of an account's balance under the
// history chart (C381): a dashed continuation of the curve, and — the part that
// actually matters — the trough and the first day the balance would go negative.
//
// The history chart answers where the account has been. The question people
// open an account to answer is where it is going, and specifically whether it
// survives until payday. Every input already existed as local recurring data;
// nothing joined it to a starting balance and drew the rest of the line.
//
// It renders nothing when there is nothing to project from. A flat line drawn
// over an account with no recurring flows is not a forecast of stability — it is
// the absence of information wearing a forecast's clothes.
func accountForwardLine(a domain.Account, balance money.Money, p acctproject.Projection, now time.Time) ui.Node {
	// Liabilities and valuation-type assets are not cash curves: a mortgage does
	// not "run out", and projecting one implies a question the number cannot
	// answer.
	if a.Class == domain.ClassLiability || isValuationType(a.Type) {
		return Fragment()
	}
	if !p.Known() {
		return Fragment()
	}
	pts := p.Series(now, forwardHorizonDays)
	if len(pts) < 2 {
		return Fragment()
	}
	series := make([]int64, 0, len(pts))
	for _, pt := range pts {
		series = append(series, pt.Balance)
	}

	low := money.New(p.Low, a.Currency)
	end := money.New(p.End, a.Currency)
	negAt, goesNegative := p.FirstNegative(now)

	// The reading comes first and the chart supports it. A shape alone leaves the
	// reader to find the trough themselves, which is the one thing they came for.
	var read ui.Node
	switch {
	case goesNegative:
		read = Span(css.Class("row-meta", "acct-fwd-warn"),
			Attr("data-testid", "acct-fwd-negative-"+a.ID),
			uistate.T("accountsFwd.negative", fmtShortDate(negAt), fmtMoney(low)))
	case p.Low < balance.Amount:
		read = Span(css.Class("row-meta"), Attr("data-testid", "acct-fwd-low-"+a.ID),
			uistate.T("accountsFwd.low", fmtMoney(low), fmtShortDate(p.LowDate), fmtMoney(end)))
	default:
		read = Span(css.Class("row-meta"), Attr("data-testid", "acct-fwd-rising-"+a.ID),
			uistate.T("accountsFwd.rising", fmtMoney(end)))
	}

	points := accountflow.Polyline(series, sparklineW, sparklineH, 2)
	var fig ui.Node = Fragment()
	if points != "" {
		fig = Svg(css.Class("acct-spark acct-fwd-spark"), Attr("data-testid", "acct-fwd-spark-"+a.ID),
			Attr("viewBox", "0 0 120 24"), Attr("width", "120"), Attr("height", "24"),
			// aria-hidden with the reading beside it in text: a line whose shape a
			// screen reader cannot perceive is decoration, and the sentence above
			// already carries every fact the chart encodes.
			Attr("aria-hidden", "true"), Attr("preserveAspectRatio", "none"),
			Polyline(Attr("points", points), Attr("fill", "none"),
				Attr("stroke", "var(--text-faint)"), Attr("stroke-width", "1.5"),
				// Dashed, and in the faint colour: this is a projection sitting next
				// to a record of what actually happened, and the two must not look
				// equally certain.
				Attr("stroke-dasharray", "3 2"),
				Attr("stroke-linejoin", "round"), Attr("stroke-linecap", "round")))
	}

	return Div(css.Class("acct-fwd"), Attr("data-testid", "acct-fwd-"+a.ID),
		Div(css.Class("acct-fwd-head"),
			Span(css.Class("row-meta", "acct-fwd-label"), uistate.T("accountsFwd.title")),
			read,
		),
		fig,
		// Say what the projection assumes. A number with an unstated assumption is
		// the thing people later call wrong.
		Span(css.Class("acct-fwd-note"), uistate.T("accountsFwd.assumes", len(p.Drivers))),
	)
}
