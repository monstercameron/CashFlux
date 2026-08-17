// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/portfolio"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// ─── FP-T1c: the return, said in words, under the balance chart ──────────────
//
// The chart above plots a BALANCE, and a balance rises for two opposite reasons
// — the market went up, or you paid more in. A doubled line is equally
// consistent with a portfolio that did nothing and an investor who doubled their
// contributions, so the chart cannot answer "how is this doing".
//
// Both returns are shown, and showing one would be worse than showing neither.
// Time-weighted answers "how did the investments do"; money-weighted answers
// "how did I do". Only the first tells someone their portfolio did well when
// their experience did not, and only the second tells them their fund is bad
// when their timing was.
//
// This is a plain render helper, not a component: it registers no hooks, so it
// can be called from inside the growth widget without adding a hook position.

// investReturnsReading renders the return figures for the charted window.
//
// vals are the portfolio's values at cutoffs (the same series the chart plots),
// so the reading and the line can never describe different periods.
func investReturnsReading(accts []domain.Account, txns []domain.Transaction,
	cutoffs []time.Time, vals []money.Money, base string) ui.Node {

	if len(cutoffs) == 0 || len(vals) != len(cutoffs) {
		return Fragment()
	}
	valuations := make([]portfolio.Valuation, 0, len(vals))
	for i := range vals {
		valuations = append(valuations, portfolio.Valuation{Date: cutoffs[i], ValueMinor: vals[i].Amount})
	}
	perf := portfolio.Returns(valuations, investExternalFlows(accts, txns))

	// FP-T1c-b: a window shorter than the floor still HAPPENED. Say what it
	// returned over the window rather than nothing — the 1M option on the chart
	// above spans fewer days than the floor, so refusing outright made it a
	// control that could never succeed. What is withheld is the YEARLY rate, and
	// the wording never says "a year".
	if !perf.TWRKnown && !perf.IRRKnown {
		if perf.PeriodKnown {
			return Div(css.Class("inv-returns"), Attr("data-testid", "invest-returns"),
				P(css.Class("t-caption", tw.TextDim), uistate.T("invret.why")),
				P(css.Class("inv-return-line"), Attr("data-testid", "invest-return-period"),
					uistate.T("invret.period", perf.PeriodPct, perf.Days)),
				P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "invest-return-noyear"),
					uistate.T("invret.noYearlyRate", portfolio.MinReturnDays)),
				P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "invest-return-basis"),
					uistate.T("invret.basis", perf.Days, perf.Valuations, perf.Flows)))
		}
		return P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "invest-return-none"),
			uistate.T("invret.tooShort", portfolio.MinReturnDays))
	}

	var twr, irr, gap ui.Node = Fragment(), Fragment(), Fragment()
	if perf.TWRKnown {
		twr = P(css.Class("inv-return-line"), Attr("data-testid", "invest-return-twr"),
			uistate.T("invret.twr", perf.TimeWeightedPct))
	}
	if perf.IRRKnown {
		irr = P(css.Class("inv-return-line"), Attr("data-testid", "invest-return-irr"),
			uistate.T("invret.irr", perf.MoneyWeightedPct))
	}
	if g, ok := perf.GapPct(); ok {
		// The gap is the honest third number: what the timing of contributions
		// cost. Naming the direction beats printing a signed figure the reader has
		// to work out the sign convention for.
		key := "invret.gapCost"
		if g >= 0 {
			key = "invret.gapGain"
		}
		if g < 0 {
			g = -g
		}
		gap = P(css.Class("inv-return-line", tw.TextDim), Attr("data-testid", "invest-return-gap"),
			uistate.T(key, g))
	}

	return Div(css.Class("inv-returns"), Attr("data-testid", "invest-returns"),
		P(css.Class("t-caption", tw.TextDim), uistate.T("invret.why")),
		twr, irr, gap,
		// Provenance, because a return with no stated basis is a number the reader
		// has no way to check or to distrust.
		P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "invest-return-basis"),
			uistate.T("invret.basis", perf.Days, perf.Valuations, perf.Flows)),
	)
}

// investExternalFlows returns money that crossed the boundary of the investment
// accounts, which is the only kind of movement that is a contribution.
//
// A transfer BETWEEN two investment accounts is not a flow — the portfolio did
// not gain a dollar, it moved one — and counting it would make rebalancing look
// like saving. A dividend or interest posting with no counterparty is likewise
// not a flow: it is the return itself, and treating it as money paid in is the
// exact inversion these figures exist to prevent.
//
// The cost of that rule: a deposit recorded as a bare credit rather than as a
// transfer is invisible here and inflates the return. Requiring a counterparty
// is still the right side to err on, because the app can see whether a transfer
// exists and cannot see what a bare credit meant — and the basis line states the
// flow count so a reader whose contributions are missing can tell.
func investExternalFlows(accts []domain.Account, txns []domain.Transaction) []portfolio.Flow {
	inSet := make(map[string]bool, len(accts))
	for _, a := range accts {
		inSet[a.ID] = true
	}
	var flows []portfolio.Flow
	for _, t := range txns {
		if !inSet[t.AccountID] || t.TransferAccountID == "" || inSet[t.TransferAccountID] {
			continue
		}
		flows = append(flows, portfolio.Flow{Date: t.Date, AmountMinor: t.Amount.Amount})
	}
	return flows
}
