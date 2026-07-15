// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/sweep"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// accounts_sweep_card.go is the UI shell for AC7 sweep-rule proposals. It mirrors
// the GL1 payday-waterfall card exactly (goals_waterfall.go): a quiet,
// preview-approve moment that appears on /accounts when a sweep rule's source
// account is over its keep amount. All computation lives in the tested
// internal/sweep + appstate.SweepProposals; approving runs the existing transfer
// flow (CreateTransferPair) and stamps the rule handled — never auto-committed.

// accountsSweepCards renders one preview-approve card per due sweep proposal, or
// Fragment() when none is due. Each card is its own component so its
// approve/dismiss hooks stay at stable render positions.
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
	nameOf := map[string]string{}
	for _, a := range app.Accounts() {
		nameOf[a.ID] = a.Name
	}
	cards := make([]ui.Node, 0, len(proposals))
	for _, p := range proposals {
		p := p
		cards = append(cards, ui.CreateElement(accountsSweepCard, accountsSweepCardProps{
			Proposal: p,
			FromName: nameOf[p.SourceAccountID],
			ToName:   nameOf[p.DestAccountID],
		}))
	}
	return Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap2, tw.Mb2), cards)
}

type accountsSweepCardProps struct {
	Proposal sweep.Proposal
	FromName string
	ToName   string
}

// accountsSweepCard renders one sweep proposal. Approve runs the transfer;
// dismiss stamps the rule so it won't re-propose until the next cadence.
func accountsSweepCard(props accountsSweepCardProps) ui.Node {
	p := props.Proposal
	hidden := ui.UseState(false)
	if hidden.Get() {
		return Fragment()
	}
	amtStr := fmtMoney(money.New(p.AmountMinor, p.Currency))

	onApprove := ui.UseEvent(func() {
		app := appstate.Default
		if _, _, err := app.CreateTransferPair(appstate.TransferParams{
			FromAccountID: p.SourceAccountID,
			ToAccountID:   p.DestAccountID,
			AmountMinor:   p.AmountMinor,
			Desc:          uistate.T("acctSweep.transferDesc", props.ToName),
		}); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		if err := app.MarkSweepProposed(p.RuleID, time.Now()); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		uistate.BumpDataRevision()
		uistate.RequestPersist()
		uistate.PostNotice(uistate.T("acctSweep.done", amtStr, props.ToName), false)
		hidden.Set(true)
	})
	onDismiss := ui.UseEvent(func() {
		if err := appstate.Default.MarkSweepProposed(p.RuleID, time.Now()); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		uistate.RequestPersist()
		hidden.Set(true)
	})

	return Div(
		css.Class("catchup-card"),
		Attr("role", "status"),
		Attr("data-testid", "acct-sweep-card-"+p.RuleID),
		Attr("aria-label", uistate.T("acctSweep.aria", props.FromName)),
		Div(css.Class("catchup-card-body", tw.Flex, tw.FlexCol),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
				Span(css.Class("catchup-card-icon"), "🧹"),
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
