// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditlog"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// TxnHistoryPanelProps configures the per-transaction history panel (#63).
type TxnHistoryPanelProps struct {
	TxnID string
}

// txnHistoryScanLimit bounds how much of the persisted audit log the panel
// scans — recent-first, so a transaction's newest changes always surface.
const txnHistoryScanLimit = 500

// TxnHistoryPanel lists everything the audit trail recorded about ONE
// transaction — edits (with field-level before → after), rule applications,
// imports, deletions — newest first. Derived entirely from the #54 audit
// change sets; entries recorded before per-entity attribution existed (or
// bulk changes touching many rows at once) won't appear, and the empty state
// says so honestly.
func TxnHistoryPanel(props TxnHistoryPanelProps) ui.Node {
	return ui.CreateElement(txnHistoryPanel, props)
}

func txnHistoryPanel(props TxnHistoryPanelProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}
	var mine []auditlog.Entry
	for _, e := range app.RecentAudit(txnHistoryScanLimit) {
		if e.EntityID == props.TxnID {
			mine = append(mine, e)
		}
	}
	// RecentAudit returns oldest-first; show newest first.
	for i, j := 0, len(mine)-1; i < j; i, j = i+1, j-1 {
		mine[i], mine[j] = mine[j], mine[i]
	}

	if len(mine) == 0 {
		return Div(css.Class("modal-scroll"), Attr("data-testid", "txn-history-panel"),
			P(css.Class("muted"), Attr("data-testid", "txn-history-empty"), uistate.T("txnhistory.empty")),
		)
	}

	rows := make([]ui.Node, 0, len(mine))
	for _, e := range mine {
		var details []ui.Node
		for _, d := range e.Details {
			details = append(details, P(css.Class("muted", tw.Text12), Attr("data-testid", "txn-history-diff"),
				d.Field+": "+d.Before+" → "+d.After))
		}
		who := e.Actor
		if who == "user" {
			who = uistate.T("txnhistory.actorYou")
		}
		rows = append(rows, Div(css.Class("row"), Attr("data-testid", "txn-history-entry"),
			Style(map[string]string{"display": "block", "padding": "0.5rem 0"}),
			Div(css.Class(tw.Flex, tw.JustifyBetween, tw.Gap2),
				Span(e.Summary),
				Span(css.Class(tw.TextFaint, tw.Text12, tw.ShrinkO), e.At.Local().Format("Jan 2, 2006 3:04 PM"))),
			P(css.Class(tw.TextFaint, tw.Text12), who),
			Div(details),
		))
	}
	return Div(css.Class("modal-scroll"), Attr("data-testid", "txn-history-panel"),
		P(css.Class("muted", tw.Text12), uistate.T("txnhistory.scopeNote")),
		Div(css.Class("rows"), rows),
	)
}
