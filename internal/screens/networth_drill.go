// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"

	"github.com/monstercameron/CashFlux/internal/attribution"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// Investigating a number without leaving the page.
//
// Detail's tables were readable and completely inert: a reader who wanted to
// know why one account moved had a single generic "View accounts" link, and
// after taking it had to find the account again by hand and reconstruct the
// question. So every row that states a number now opens, in place, to the facts
// behind it — and only then offers a link, aimed at the specific thing the
// reader was already looking at.
//
// Each expandable row is its OWN component: On* prop options register hooks, so
// a per-row handler cannot be created inside a variable-length loop. The row
// renders as its own <tbody> holding the summary row plus the panel row, which
// keeps the table valid HTML (several tbody elements per table are allowed,
// a <details> inside a <tr> is not).

// nwsDrillRowProps drives one expandable table row.
type nwsDrillRowProps struct {
	// TestID marks the summary row so the suite can find it.
	TestID string
	// Data is an optional data-* discriminator (the leg kind, the account id).
	DataKey, DataVal string
	// Cells are the visible row's cells, in order. The first one gains the
	// disclosure triangle.
	Cells []ui.Node
	// Panel is the content revealed beneath, and Span is how many columns it
	// stretches across.
	Panel ui.Node
	Span  int
	// Label names what is being opened, for the toggle's accessible name.
	Label string
}

// nwsDrillRow renders one row that opens in place.
func nwsDrillRow(p nwsDrillRowProps) ui.Node {
	open := ui.UseState(false)
	toggle := ui.UseEvent(Prevent(func() { open.Set(!open.Get()) }))

	cells := make([]any, 0, len(p.Cells)+1)
	for i, c := range p.Cells {
		if i == 0 {
			cells = append(cells, Td(
				Button(ClassStr("nws-drill-toggle"+If2(open.Get(), " is-open", "")), Type("button"),
					Attr("data-testid", p.TestID+"-toggle"),
					Attr("aria-expanded", boolStr(open.Get())),
					Attr("aria-label", uistate.T("nws.drillAria", p.Label)),
					Title(uistate.T("nws.drillAria", p.Label)),
					OnClick(toggle),
					Span(css.Class("nws-drill-caret"), Attr("aria-hidden", "true"), "›"),
					Span(c),
				),
			))
			continue
		}
		cells = append(cells, c)
	}

	rowArgs := []any{css.Class("nws-drill-row"), Attr("data-testid", p.TestID)}
	if p.DataKey != "" {
		rowArgs = append(rowArgs, Attr(p.DataKey, p.DataVal))
	}
	rowArgs = append(rowArgs, cells...)

	return Tbody(css.Class("nws-drill"),
		Tr(rowArgs...),
		If(open.Get(), Tr(css.Class("nws-drill-panel-row"),
			Td(Attr("colspan", strconv.Itoa(p.Span)), Attr("data-testid", p.TestID+"-panel"),
				Div(css.Class("nws-drill-panel"), p.Panel)),
		)),
	)
}

// nwsFact renders one labelled fact inside a drill panel.
func nwsFact(label, value string) ui.Node {
	return Div(css.Class("nws-fact"),
		Span(css.Class("nws-fact-k"), label),
		Span(css.Class("nws-fact-v"), value),
	)
}

// nwsAccountPanel is the investigation panel for one account: where the balance
// stands, where it came from, and what actually moved it over the window —
// followed by links aimed at THIS account rather than at the account list.
func nwsAccountPanel(r nwsAcctRow, v nwsView) ui.Node {
	facts := []ui.Node{
		nwsFact(uistate.T("nws.factKind"), selectorTypeLabel(r.Acct.Type)),
		nwsFact(uistate.T("nws.factOpening"), fmtMoney(money.New(r.StartMinor, v.Base))),
		nwsFact(uistate.T("nws.factClosing"), fmtMoney(money.New(r.EndMinor, v.Base))),
	}
	// The split that actually answers "why did this move": money that passed
	// through the account, versus balance the household simply asserted.
	if r.FlowMinor != 0 {
		facts = append(facts, nwsFact(uistate.T("nws.factFlow"), nwsSigned(r.FlowMinor, v.Base)))
	}
	if r.AdjMinor != 0 {
		facts = append(facts, nwsFact(uistate.T("nws.factAdjusted"), nwsSigned(r.AdjMinor, v.Base)))
	}
	if r.Acct.Currency != "" && r.Acct.Currency != v.Base {
		facts = append(facts, nwsFact(uistate.T("nws.factCurrency"), r.Acct.Currency))
	}
	// Where the balance came from and when it was last confirmed — the account's
	// own trustworthiness, stated per account rather than only in aggregate.
	source := uistate.T("nws.factSourceTracked")
	if r.Manual {
		source = uistate.T("nws.factSourceManual")
	}
	facts = append(facts, nwsFact(uistate.T("nws.factSource"), source))
	confirmed := uistate.T("nws.dqNever")
	if !r.AsOf.IsZero() {
		confirmed = uistate.LoadPrefs().FormatDate(r.AsOf)
	}
	facts = append(facts, nwsFact(uistate.T("nws.factConfirmed"), confirmed))
	if r.Acct.InterestRateAPR > 0 {
		facts = append(facts, nwsFact(uistate.T("nws.factRate"), strconv.FormatFloat(r.Acct.InterestRateAPR, 'f', -1, 64)+"%"))
	}

	return Fragment(
		Div(css.Class("nws-facts"), facts),
		Div(css.Class("nws-drill-actions"),
			ui.CreateElement(nwsLedgerLink, nwsLedgerLinkProps{
				AccountID: r.Acct.ID, Name: r.Acct.Name,
				From: v.Since.Format(dateutil.Layout),
				To:   v.Until.AddDate(0, 0, -1).Format(dateutil.Layout),
			}),
			A(css.Class("btn", "btn-sm"), Href(uistate.RoutePath("/accounts")),
				Attr("data-testid", "nws-drill-account"), uistate.T("nws.factOpenAccount")),
		),
	)
}

// nwsLedgerLinkProps drives the "see the transactions" jump.
type nwsLedgerLinkProps struct {
	AccountID, Name string
	From, To        string
}

// nwsLedgerLink opens the ledger already filtered to THIS account over the
// window on screen — the difference between offering a drill-down and offering
// a search. Own component so the navigate hook sits at a stable call-site.
func nwsLedgerLink(p nwsLedgerLinkProps) ui.Node {
	nav := router.UseNavigate()
	filterAtom := uistate.UseTxFilter()
	go2 := ui.UseEvent(Prevent(func() {
		f := uistate.TxFilter{From: p.From, To: p.To, Account: p.AccountID}.Normalize()
		filterAtom.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}))
	return Button(css.Class("btn", "btn-sm"), Type("button"),
		Attr("data-testid", "nws-drill-ledger"),
		Title(uistate.T("nws.factLedgerTitle", p.Name)),
		OnClick(go2), uistate.T("nws.factLedger"))
}

// nwsLegPanel is the next level of "why": which accounts produced this leg. The
// figures come straight from the bridge's own contributor lists, so they sum to
// the leg by construction rather than by a second computation that could drift.
func nwsLegPanel(k attribution.LegKind, v nwsView) ui.Node {
	cs := v.Bridge.Contributors(k)
	if len(cs) == 0 {
		// The residual is the one leg with nothing to open: it is precisely what
		// could not be attributed to an account, and saying so is the point.
		if k == attribution.LegResidual {
			return P(css.Class("nws-drill-note"), Attr("data-testid", "nws-leg-none"),
				uistate.T("nws.legResidualWhat"))
		}
		return P(css.Class("nws-drill-note"), uistate.T("nws.legNoContributors"))
	}
	facts := make([]ui.Node, 0, len(cs))
	for _, c := range cs {
		facts = append(facts, Div(css.Class("nws-fact"), Attr("data-testid", "nws-leg-contributor"),
			Span(css.Class("nws-fact-k"), c.AccountName),
			Span(css.Class("nws-fact-v"), nwsSigned(c.AmountMinor, v.Base)),
		))
	}
	return Fragment(
		P(css.Class("nws-drill-note"), uistate.T("nws.legContributors", nwsLegLabel(k))),
		Div(css.Class("nws-facts"), facts),
	)
}

// nwsSigned formats a signed movement with an explicit sign.
func nwsSigned(minor int64, base string) string {
	sign := "+"
	if minor < 0 {
		sign = "−"
	}
	return sign + fmtMoney(money.New(absMinor(minor), base))
}
