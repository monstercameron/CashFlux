// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// goals_waterfall.go is the UI shell for the payday waterfall (GL1): the quiet,
// dismissible preview-approve card that appears on /goals when income has landed
// since the last handled waterfall. All computation lives in the tested
// internal/waterfall + appstate.WaterfallPlan; this file only renders and, on the
// user's explicit approval, calls appstate.ApplyWaterfall (which writes virtual
// earmarks — never a transaction, never auto-committed).

// waterfallAmount formats a base-currency minor amount for the card in the same
// accounting style as every other figure on screen (symbol + thousands grouping)
// — never the bare-decimal editable-input form.
func waterfallAmount(minor int64, base string) string {
	return fmtMoney(money.New(minor, base))
}

// goalsWaterfallCard renders the payday funding card, or Fragment() when there is
// no fresh income or no fundable goal. One sentence, the priority-ordered plan,
// one primary action ("Fund goals ($X)"), one "Not now" — a low-pressure moment,
// never naggy. Its own component so the approval/dismiss hooks stay at stable
// render positions.
func goalsWaterfallCard() ui.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	_ = uistate.UseDataRevision().Get()

	// Local hide-state so a click hides the card instantly (the handled stamp is
	// also persisted so it never returns for this income).
	hidden := ui.UseState(false)
	if hidden.Get() {
		return Fragment()
	}

	now := time.Now()
	proposal := app.WaterfallPlan(now)
	if !proposal.HasProposal() {
		return Fragment()
	}

	base := proposal.Currency
	incomeStr := waterfallAmount(proposal.IncomeMinor, base)
	fundedStr := waterfallAmount(proposal.FundedMinor, base)

	onApprove := ui.UseEvent(func() {
		if err := app.ApplyWaterfall(proposal, now); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		uistate.BumpDataRevision()
		uistate.RequestPersist()
		uistate.PostNotice(uistate.T("goals.waterfallDone", fundedStr), false)
		hidden.Set(true)
	})
	onDismiss := ui.UseEvent(func() {
		if err := app.DismissWaterfall(now); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		uistate.RequestPersist()
		hidden.Set(true)
	})

	// Priority-ordered funding lines. Name on the left, amount right-aligned in its own
	// column (tabular figures) so the dollar amounts line up regardless of goal-name
	// length — no ragged trailing edge.
	lineNodes := make([]ui.Node, 0, len(proposal.Lines))
	for _, l := range proposal.Lines {
		lineNodes = append(lineNodes, Li(css.Class("wf-line"),
			Span(css.Class("wf-line-name"), l.GoalName),
			Span(css.Class("wf-line-amt"), waterfallAmount(l.AmountMinor, base))))
	}

	var remainderNode ui.Node = Fragment()
	if proposal.RemainderMinor > 0 {
		remainderNode = P(css.Class("t-caption", tw.TextDim),
			uistate.T("goals.waterfallRemainder", waterfallAmount(proposal.RemainderMinor, base)))
	}

	return Div(
		css.Class("catchup-card"),
		Attr("role", "status"),
		Attr("data-testid", "goals-waterfall-card"),
		Attr("aria-label", uistate.T("goals.waterfallAria")),
		// ItemsStart: this card lays out as a column, so left-align every row — the shared
		// .catchup-card-body centers children (for its single-row variant), which would
		// otherwise center the plan lines + action buttons against the left-aligned intro.
		Div(css.Class("catchup-card-body", tw.Flex, tw.FlexCol, tw.ItemsStart),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
				Span(css.Class("catchup-card-icon"), "💧"),
				Div(css.Class("catchup-card-text"),
					Strong(uistate.T("goals.waterfallTitle")),
					P(uistate.T("goals.waterfallBody", incomeStr)),
				),
			),
			Ul(css.Class("wf-lines", tw.Mt2, tw.FlexCol, tw.Gap1), Attr("data-testid", "goals-waterfall-lines"), lineNodes),
			remainderNode,
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt3),
				Button(css.Class("btn btn-primary btn-sm"), Type("button"),
					Attr("data-testid", "goals-waterfall-approve"), OnClick(onApprove),
					uistate.T("goals.waterfallApprove", fundedStr)),
				Button(css.Class("btn btn-ghost btn-sm"), Type("button"),
					Attr("data-testid", "goals-waterfall-dismiss"), OnClick(onDismiss),
					uistate.T("goals.waterfallDismiss")),
			),
		),
	)
}
