// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditlog"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// txnHistoryTimeframeKV persists the popover's chosen timeframe window (feedback
// #7) so it holds across reopens. Empty/"all" means no window (show everything).
const txnHistoryTimeframeKV = "txnhist.timeframe"

// txnHistoryTimeframe reads the persisted timeframe as a day count (0 = all time).
func txnHistoryTimeframe() int {
	switch uistate.KVGet(txnHistoryTimeframeKV) {
	case "90":
		return 90
	case "30":
		return 30
	case "7":
		return 7
	}
	return 0
}

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
	// Configurable timeframe (feedback #7): the history can be scoped to a recent
	// window instead of always showing everything. Persisted so it holds across reopens.
	tf := ui.UseState(txnHistoryTimeframe())
	setTf := func(days int) {
		v := "all"
		if days > 0 {
			v = strconv.Itoa(days)
		}
		uistate.KVSet(txnHistoryTimeframeKV, v)
		uistate.RequestPersist()
		tf.Set(days)
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
	// Apply the timeframe window (0 = all time).
	days := tf.Get()
	if days > 0 {
		cutoff := time.Now().AddDate(0, 0, -days)
		kept := make([]auditlog.Entry, 0, len(mine))
		for _, e := range mine {
			if e.At.After(cutoff) {
				kept = append(kept, e)
			}
		}
		mine = kept
	}

	// Timeframe selector — each option is its own child component so its click hook
	// stays at a stable render position (not inside a loop).
	tfControl := Div(css.Class("txnhist-tf", tw.Flex, tw.ItemsCenter, tw.Gap2, tw.FlexWrap, tw.Mb2),
		Span(css.Class("t-caption", tw.TextDim), uistate.T("txnhistory.timeframe")),
		ui.CreateElement(txnHistTfButton, txnHistTfProps{Label: uistate.T("txnhistory.tfAll"), Days: 0, Selected: days == 0, OnPick: setTf}),
		ui.CreateElement(txnHistTfButton, txnHistTfProps{Label: uistate.T("txnhistory.tf90"), Days: 90, Selected: days == 90, OnPick: setTf}),
		ui.CreateElement(txnHistTfButton, txnHistTfProps{Label: uistate.T("txnhistory.tf30"), Days: 30, Selected: days == 30, OnPick: setTf}),
		ui.CreateElement(txnHistTfButton, txnHistTfProps{Label: uistate.T("txnhistory.tf7"), Days: 7, Selected: days == 7, OnPick: setTf}),
	)

	// A follow-up-task action right here in the history panel (coworker feedback): seed the add-task
	// modal with a suggested title + this transaction pre-linked, matching the row ⋯-menu action.
	merchant := ""
	for _, t := range app.Transactions() {
		if t.ID == props.TxnID {
			merchant = firstNonEmpty(t.Payee, t.Desc)
			break
		}
	}
	onFollowUp := ui.UseEvent(func() {
		uistate.SetTaskAddSeed(uistate.TaskAddSeed{
			Title:    uistate.T("transactions.followUpTaskTitle", merchant),
			LinkType: string(domain.RelatedTransaction),
			LinkID:   props.TxnID,
		})
		uistate.SetAddTarget("task")
	})
	followUpBtn := Div(css.Class(tw.Flex, tw.JustifyEnd, tw.Mb2),
		Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "txn-history-followup"),
			OnClick(onFollowUp), uistate.T("transactions.followUpTask")))

	if len(mine) == 0 {
		emptyKey := "txnhistory.empty"
		if days > 0 {
			emptyKey = "txnhistory.emptyForWindow"
		}
		return Div(css.Class("modal-scroll"), Attr("data-testid", "txn-history-panel"),
			followUpBtn,
			tfControl,
			P(css.Class("muted"), Attr("data-testid", "txn-history-empty"), uistate.T(emptyKey)),
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
		followUpBtn,
		tfControl,
		P(css.Class("muted", tw.Text12), uistate.T("txnhistory.scopeNote")),
		Div(css.Class("rows"), rows),
	)
}

// txnHistTfProps configures one timeframe-filter option in the history popover.
type txnHistTfProps struct {
	Label    string
	Days     int // 0 = all time
	Selected bool
	OnPick   func(days int)
}

// txnHistTfButton is its own component so each option's click hook stays at a
// stable render position (the On*-in-loop gotcha).
func txnHistTfButton(props txnHistTfProps) ui.Node {
	cls := "btn btn-sm"
	if props.Selected {
		cls += " btn-accent"
	}
	return Button(css.Class(cls), Type("button"), Attr("data-testid", "txn-history-tf"),
		Attr("aria-pressed", ariaBool(props.Selected)),
		OnClick(func() { props.OnPick(props.Days) }), props.Label)
}
