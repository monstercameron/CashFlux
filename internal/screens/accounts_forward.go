// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/accountflow"
	"github.com/monstercameron/CashFlux/internal/acctproject"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// forwardHorizonDays is how far the account-detail projection looks. Ninety days
// is roughly a quarter: far enough to cross several pay cycles and the quarterly
// bills, near enough that "today's recurring set continues" is still a
// defensible assumption rather than a guess dressed as a forecast.
const forwardHorizonDays = 90

// forwardUnavailable is the status line shown when the forward projection cannot be computed.
func forwardUnavailable(a domain.Account) ui.Node {
	return P(css.Class("row-meta"), Role("status"), Attr("data-testid", "acct-fwd-unavailable-"+a.ID), uistate.T("calculation.unavailable"))
}

// accountForwardLine renders the next 90 days of an account's balance under the
// history chart: a dashed continuation of the curve, and the two facts that matter
// most, the trough and the first day the balance would go negative.
//
// The history chart answers where the account has been. This line answers whether
// it survives until payday, so the reading comes first and the chart supports it.
// The projection is built from the account's recurring flows, and the line says how
// many flows it assumes keep running.
//
// It renders nothing when there is nothing to project from. A flat line over an
// account with no recurring flows would claim a stability the app has no evidence
// for.
func accountForwardLine(a domain.Account, balance money.Money, p acctproject.Projection, projectionErr error, now time.Time) ui.Node {
	// Liabilities and valuation-type assets are not cash curves: a mortgage does
	// not "run out", and projecting one implies a question the number cannot
	// answer.
	if !a.ShowsCashCurve() {
		return Fragment()
	}
	if projectionErr != nil {
		return forwardUnavailable(a)
	}
	if !p.Known() {
		return Fragment()
	}
	pts, err := p.Series(now, forwardHorizonDays)
	if err != nil {
		return forwardUnavailable(a)
	}
	if len(pts) < 2 {
		return Fragment()
	}
	series := make([]int64, 0, len(pts))
	for _, pt := range pts {
		series = append(series, pt.Balance)
	}

	low := money.New(p.Low, a.Currency)
	end := money.New(p.End, a.Currency)
	outlook, err := p.Outlook(balance.Amount, now)
	if err != nil {
		return forwardUnavailable(a)
	}

	// The reading comes first and the chart supports it. A shape alone leaves the
	// reader to find the trough themselves, which is the one thing they came for.
	var read ui.Node
	switch outlook.Kind {
	case acctproject.OutlookNegative:
		read = Span(css.Class("row-meta", "acct-fwd-warn"),
			Attr("data-testid", "acct-fwd-negative-"+a.ID),
			uistate.T("accountsFwd.negative", fmtShortDate(outlook.NegativeOn), fmtMoney(low)))
	case acctproject.OutlookLow:
		read = Span(css.Class("row-meta"), Attr("data-testid", "acct-fwd-low-"+a.ID),
			uistate.T("accountsFwd.low", fmtMoney(low), fmtShortDate(p.LowDate), fmtMoney(end)))
	default:
		read = Span(css.Class("row-meta"), Attr("data-testid", "acct-fwd-rising-"+a.ID),
			uistate.T("accountsFwd.risingOrFlat", fmtMoney(end)))
	}

	points := accountflow.Polyline(series, sparklineW, sparklineH, 2)
	width := strconv.FormatFloat(sparklineW, 'f', -1, 64)
	height := strconv.FormatFloat(sparklineH, 'f', -1, 64)
	fig := If(points != "", Svg(css.Class("acct-spark acct-fwd-spark"), Attr("data-testid", "acct-fwd-spark-"+a.ID),
		Attr("viewBox", "0 0 "+width+" "+height), Attr("width", width), Attr("height", height),
		// aria-hidden with the reading beside it in text: a line whose shape a
		// screen reader cannot perceive is decoration, and the sentence above
		// already carries every fact the chart encodes.
		AriaHidden(true), Attr("preserveAspectRatio", "none"),
		Polyline(Attr("points", points), Attr("fill", "none"),
			Attr("stroke", "var(--text-faint)"), Attr("stroke-width", "1.5"),
			// Dashed, and in the faint colour: this is a projection sitting next
			// to a record of what actually happened, and the two must not look
			// equally certain.
			Attr("stroke-dasharray", "3 2"),
			Attr("stroke-linejoin", "round"), Attr("stroke-linecap", "round"))))

	return Div(css.Class("acct-fwd"), Attr("data-testid", "acct-fwd-"+a.ID),
		Div(css.Class("acct-fwd-head"),
			Span(css.Class("row-meta", "acct-fwd-label"), uistate.T("accountsFwd.title")),
			read,
		),
		fig,
		// Say what the projection assumes. A number with an unstated assumption is
		// the thing people later call wrong.
		Span(css.Class("acct-fwd-note"), uistate.TN("accountsFwd.assumesOne", "accountsFwd.assumesMany", len(p.Drivers))),
	)
}
