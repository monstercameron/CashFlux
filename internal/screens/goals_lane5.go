// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// goals_lane5.go holds the #65 goals refinements: the goal-conflict warning
// strip (several goals' earmarks claiming the same account balance), the
// "fund from next paycheck" preview, and the payday funding-order control.

// computeGoalConflicts reuses the household native reservation snapshot. Unknown
// claimed positions remain visible, and duplicate rows never imply two goals.
func computeGoalConflicts(app *appstate.App) []goals.AccountReservation {
	if app == nil {
		return nil
	}
	return goalReservations(app).Warnings(2)
}

func goalConflictStrip(_ struct{}) ui.Node {
	_ = uistate.UseDataRevision().Get()
	conflicts := computeGoalConflicts(appstate.Default)
	if len(conflicts) == 0 {
		return Fragment()
	}
	return ui.CreateElement(goalConflictDetails, conflicts)
}

func goalConflictDetails(conflicts []goals.AccountReservation) ui.Node {
	viewAtom := uistate.UseGoalsView()
	goEarmarks := ui.UseEvent(Prevent(func() { viewAtom.Set(uistate.GoalsViewEarmarks) }))
	rows := MapKeyed(conflicts, func(p goals.AccountReservation) any { return p.Account.ID }, func(p goals.AccountReservation) ui.Node {
		return Div(css.Class("budget-issue-row"), Attr("data-testid", "goal-conflict-"+p.Account.ID),
			ui.CreateElement(reservationNotice, reservationNoticeProps{Position: p, ShowGoalNames: true}))
	})
	return Div(css.Class("budget-issues-wrap"), Attr("data-testid", "goals-conflict-strip"),
		Div(css.Class("budget-issues-detail"), rows,
			Button(css.Class("btn btn-sm", tw.Mt2), Type("button"), Attr("data-testid", "goals-conflict-review"),
				Title(uistate.T("goals.conflictReviewTitle")), OnClick(goEarmarks), uistate.T("goals.conflictReview"))))
}

// goalsPaycheckPreviewCard shows what the payday waterfall WOULD do with the
// next paycheck (#65), before any income lands — an estimate-labelled preview
// whose approval remains the real waterfall card the moment income arrives.
// Renders nothing while a live proposal exists (that card owns the moment).
func goalsPaycheckPreviewCard(_ struct{}) ui.Node {
	_ = uistate.UseDataRevision().Get()
	open := ui.UseState(false)
	toggle := ui.UseEvent(Prevent(func() { open.Set(!open.Get()) }))
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	now := time.Now()
	if app.WaterfallPlan(now).HasProposal() {
		return Fragment()
	}
	est := app.EstimatedNextPaycheckMinor(now)
	if est <= 0 {
		return Fragment()
	}
	preview := app.WaterfallPreview(now, est)
	if !preview.HasProposal() {
		return Fragment()
	}
	base := preview.Currency
	head := Button(css.Class("goal-plan-toggle"), Type("button"),
		Attr("data-testid", "goals-paycheck-preview-toggle"),
		Attr("aria-expanded", ariaBool(open.Get())), OnClick(toggle),
		uiw.Icon(icon.TrendingUp, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
		Span(uistate.T("goals.paycheckPreviewToggle", fmtMoney(money.New(est, base)))))
	if !open.Get() {
		return Div(css.Class(tw.Mt2), head)
	}
	var lines []ui.Node
	for _, l := range preview.Lines {
		lines = append(lines, Li(css.Class("wf-line"),
			Span(css.Class("wf-line-name"), l.GoalName),
			Span(css.Class("wf-line-amt"), fmtMoney(money.New(l.AmountMinor, base)))))
	}
	return Div(css.Class(tw.Mt2), head,
		Div(css.Class("catchup-card", tw.Mt2), Attr("data-testid", "goals-paycheck-preview"),
			Div(css.Class("catchup-card-body", tw.Flex, tw.FlexCol, tw.ItemsStart),
				P(css.Class("budget-sub"), uistate.T("goals.paycheckPreviewIntro", fmtMoney(money.New(est, base)))),
				Ul(css.Class("wf-lines", tw.Mt2), lines),
				If(preview.RemainderMinor > 0, P(css.Class("budget-sub"),
					uistate.T("goals.waterfallRemainder", fmtMoney(money.New(preview.RemainderMinor, base))))),
				P(css.Class("budget-sub"), Attr("data-testid", "goals-paycheck-preview-note"),
					uistate.T("goals.paycheckPreviewNote")),
			),
		),
	)
}

// goalsFundingOrderCard is the funding-priority control (#65): the fundable
// goals in the exact order the payday waterfall will fill them, each with
// move-up/move-down controls (fully keyboard-accessible reordering). Collapsed
// behind a disclosure so the goals page stays calm.
func goalsFundingOrderCard(_ struct{}) ui.Node {
	_ = uistate.UseDataRevision().Get()
	open := ui.UseState(false)
	toggle := ui.UseEvent(Prevent(func() { open.Set(!open.Get()) }))
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	now := time.Now()
	// The same eligibility filter the waterfall applies, so this list IS the
	// funding sequence — never a superset that implies paused goals get funded.
	var fundable []domain.Goal
	for _, g := range goals.FundingOrdered(app.Goals()) {
		if g.Archived || !g.IsFinancial() || g.IsPaused(now) {
			continue
		}
		if complete, err := goals.IsComplete(g); err != nil || complete {
			continue
		}
		fundable = append(fundable, g)
	}
	if len(fundable) < 2 {
		return Fragment()
	}
	move := func(id string, delta int) {
		plan, ok := goals.MoveFunding(app.Goals(), id, delta)
		if !ok {
			return
		}
		for _, g := range app.Goals() {
			want, has := plan[g.ID]
			if !has || g.FundingOrder == want {
				continue
			}
			g.FundingOrder = want
			_ = app.PutGoal(g)
		}
		uistate.BumpDataRevision()
		uistate.RequestPersist()
	}
	head := Button(css.Class("goal-plan-toggle"), Type("button"),
		Attr("data-testid", "goals-funding-order-toggle"),
		Attr("aria-expanded", ariaBool(open.Get())), OnClick(toggle),
		uiw.Icon(icon.List, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
		Span(uistate.T("goals.fundingOrderToggle")))
	if !open.Get() {
		return Div(css.Class(tw.Mt2), head)
	}
	rows := make([]ui.Node, 0, len(fundable))
	for i, g := range fundable {
		rows = append(rows, ui.CreateElement(goalFundingOrderRow, goalFundingOrderRowProps{
			ID: g.ID, Name: g.Name, Pos: i + 1, First: i == 0, Last: i == len(fundable)-1, OnMove: move,
		}))
	}
	return Div(css.Class(tw.Mt2), head,
		Div(css.Class("catchup-card", tw.Mt2), Attr("data-testid", "goals-funding-order"),
			Div(css.Class("catchup-card-body", tw.Flex, tw.FlexCol, tw.ItemsStart),
				P(css.Class("budget-sub"), uistate.T("goals.fundingOrderIntro")),
				Div(css.Class(tw.FlexCol, tw.Gap1, tw.Mt2), Style(map[string]string{"display": "flex", "width": "100%"}), rows),
			),
		),
	)
}

// goalFundingOrderRowProps drives one row of the funding-order list.
type goalFundingOrderRowProps struct {
	ID     string
	Name   string
	Pos    int
	First  bool
	Last   bool
	OnMove func(id string, delta int)
}

// goalFundingOrderRow is one goal in the funding sequence with its position and
// up/down movers. Its own component so no click hook registers inside a loop.
func goalFundingOrderRow(props goalFundingOrderRowProps) ui.Node {
	up := ui.UseEvent(Prevent(func() { props.OnMove(props.ID, -1) }))
	down := ui.UseEvent(Prevent(func() { props.OnMove(props.ID, 1) }))
	return Div(css.Class("wf-line"), Attr("data-testid", "goal-funding-row-"+props.ID),
		Span(css.Class("wf-line-amt"), fmt.Sprintf("%d.", props.Pos)),
		Span(css.Class("wf-line-name"), props.Name),
		Div(css.Class(tw.InlineFlex, tw.ItemsCenter, tw.Gap1),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "goal-funding-up-"+props.ID),
				Attr("aria-label", uistate.T("goals.fundingMoveUp", props.Name)), Title(uistate.T("goals.fundingMoveUp", props.Name)),
				DisabledIf(props.First), OnClick(up), uiw.Icon(icon.ChevronUp, css.Class(tw.W35, tw.H35))),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "goal-funding-down-"+props.ID),
				Attr("aria-label", uistate.T("goals.fundingMoveDown", props.Name)), Title(uistate.T("goals.fundingMoveDown", props.Name)),
				DisabledIf(props.Last), OnClick(down), uiw.Icon(icon.ChevronDown, css.Class(tw.W35, tw.H35))),
		),
	)
}
