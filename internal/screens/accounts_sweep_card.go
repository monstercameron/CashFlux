// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/sweep"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// The sweep cards are the UI for sweep-rule proposals. A proposal appears on
// /accounts when a rule's source account holds more than its keep amount. The card
// follows the same pattern as the payday-waterfall card on /goals (goals_waterfall.go):
// a quiet preview-and-approve moment. The math lives in internal/sweep and
// appstate.SweepProposals. Approving runs the usual transfer flow
// (CreateTransferPair) and marks the rule handled. Nothing moves without approval.

// accountsSweepCards renders one card per due sweep proposal, or Fragment() when none
// is due. Each card is its own component so its approve and dismiss hooks keep stable
// positions.
func accountsSweepCards() ui.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	_ = uistate.UseDataRevision().Get()

	proposals := app.SweepProposals(time.Now())
	if len(proposals) == 0 {
		return Fragment()
	}
	names := map[string]string{}
	for _, a := range app.Accounts() {
		names[a.ID] = a.Name
	}
	// A proposal whose account no longer exists reads "Unknown account", not a blank name.
	nameOf := func(accountID string) string {
		if n := names[accountID]; n != "" {
			return n
		}
		return uistate.T("acctSweepCfg.unknownAccount")
	}
	cards := make([]ui.Node, 0, len(proposals))
	for _, p := range proposals {
		cards = append(cards, WithKey(ui.CreateElement(accountsSweepCard, accountsSweepCardProps{
			Proposal: p,
			FromName: nameOf(p.SourceAccountID),
			ToName:   nameOf(p.DestAccountID),
		}), p.RuleID))
	}
	return Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap2, tw.Mb2), cards)
}

type accountsSweepCardProps struct {
	Proposal sweep.Proposal
	FromName string
	ToName   string
}

// accountsSweepCard renders one sweep proposal. Approve runs the transfer; dismiss
// marks the rule handled so it does not propose again until its next cadence.
func accountsSweepCard(props accountsSweepCardProps) ui.Node {
	app := appstate.Default
	p := props.Proposal
	hidden := ui.UseState(false)
	amtStr := fmtMoney(money.New(p.AmountMinor, p.Currency))

	onApprove := ui.UseEvent(func() {
		err := app.ApproveSweepProposal(p,
			uistate.T("acctSweep.transferDesc", props.ToName), time.Now())
		switch {
		case err == nil:
			uistate.BumpDataRevision()
			uistate.RequestPersist()
			uistate.PostNotice(uistate.T("acctSweep.done", amtStr, props.ToName), false)
			hidden.Set(true)
		case errors.Is(err, appstate.ErrSweepPostedUnmarked):
			// The money has moved, so the card must not be offered again.
			uistate.BumpDataRevision()
			uistate.RequestPersist()
			postAcctWriteError("approve sweep", err)
			hidden.Set(true)
		case errors.Is(err, appstate.ErrSweepAlreadyPosted):
			// The rule's earlier sweep in this window stands and nothing new moved, so say that
			// instead of reporting this proposal's amount; the rebuilt list drops the card.
			uistate.BumpDataRevision()
			uistate.RequestPersist()
			uistate.PostNotice(uistate.T("acctSweep.alreadyPosted"), false)
			hidden.Set(true)
		case errors.Is(err, appstate.ErrSweepProposalStale):
			// The proposals are rebuilt from the current balances and rules, so the stale card goes away.
			uistate.BumpDataRevision()
			postAcctWriteError("approve sweep", err)
		default:
			postAcctWriteError("approve sweep", err)
		}
	})
	onDismiss := ui.UseEvent(func() {
		if err := app.MarkSweepProposed(p.RuleID, time.Now()); err != nil {
			postAcctWriteError("dismiss sweep", err)
			return
		}
		uistate.RequestPersist()
		hidden.Set(true)
	})

	if hidden.Get() {
		return Fragment()
	}
	return Div(
		css.Class("catchup-card"),
		Role("group"),
		Attr("data-testid", "acct-sweep-card-"+p.RuleID),
		AriaLabel(uistate.T("acctSweep.aria", props.FromName)),
		Div(css.Class("catchup-card-body", tw.Flex, tw.FlexCol),
			// The live region holds only the text. The approve and dismiss buttons sit outside it so a
			// screen reader does not announce them as part of the suggestion.
			Div(Role("status"),
				Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
					Span(css.Class("catchup-card-icon"), AriaHidden(true), "🧹"),
					Div(css.Class("catchup-card-text"),
						Strong(uistate.T("acctSweep.title")),
						P(uistate.T("acctSweep.body", amtStr, props.FromName, props.ToName)),
					),
				),
				// Explainable breakdown: balance − keep − earmarked = the excess to move.
				P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "acct-sweep-breakdown-"+p.RuleID),
					uistate.T("acctSweep.breakdown",
						fmtMoney(money.New(p.BalanceMinor, p.Currency)),
						fmtMoney(money.New(p.KeepMinor, p.Currency)),
						fmtMoney(money.New(p.EarmarkedMinor, p.Currency)))),
			),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt3),
				Button(css.Class("btn btn-primary btn-sm"), Type("button"),
					Attr("data-testid", "acct-sweep-approve-"+p.RuleID), OnClick(onApprove),
					uistate.T("acctSweep.approve", amtStr)),
				Button(css.Class("btn btn-ghost btn-sm"), Type("button"),
					Attr("data-testid", "acct-sweep-dismiss-"+p.RuleID), OnClick(onDismiss),
					uistate.T("acctSweep.dismiss")),
			),
		),
	)
}
