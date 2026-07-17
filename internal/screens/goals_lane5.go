// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// goals_lane5.go holds the #65 goals refinements: the goal-conflict warning
// strip (several goals' earmarks claiming the same account balance), the
// "fund from next paycheck" preview, and the payday funding-order control.

// goalConflict is one over-claimed account: which goals share it and by how much
// the combined earmarks exceed the balance (base minor units).
type goalConflict struct {
	AccountID   string
	AccountName string
	GoalNames   []string
	ClaimMinor  int64
	BalMinor    int64
}

// computeGoalConflicts finds accounts where MULTIPLE goals' earmarks together
// exceed the account's balance — the shared-claim case the per-card overbooked
// flag can't explain on its own. Single-goal overbooking stays a card concern.
func computeGoalConflicts(app *appstate.App) []goalConflict {
	if app == nil {
		return nil
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	toBase := func(m money.Money) int64 {
		if m.Currency == base || m.Currency == "" {
			return m.Amount
		}
		if conv, err := rates.Convert(m, base); err == nil {
			return conv.Amount
		}
		return m.Amount
	}
	claim := map[string]int64{}
	names := map[string][]string{}
	for _, g := range app.Goals() {
		if g.Archived {
			continue
		}
		for _, al := range g.Allocations {
			if al.AccountID == "" || al.Amount.Amount <= 0 {
				continue
			}
			claim[al.AccountID] += toBase(al.Amount)
			names[al.AccountID] = append(names[al.AccountID], g.Name)
		}
	}
	txns := app.Transactions()
	var out []goalConflict
	for _, a := range app.Accounts() {
		if len(names[a.ID]) < 2 || claim[a.ID] <= 0 {
			continue
		}
		bal, _ := ledger.Balance(a, txns)
		if claim[a.ID] > toBase(bal) {
			out = append(out, goalConflict{
				AccountID: a.ID, AccountName: a.Name,
				GoalNames: names[a.ID], ClaimMinor: claim[a.ID], BalMinor: toBase(bal),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AccountName < out[j].AccountName })
	return out
}

// goalConflictStrip is the warning banner over the goals list when goals' plans
// claim the same balance (#65). One row per over-claimed account, naming the
// goals, the combined claim, the balance, and the gap — with a jump into the
// earmarks manager where the claims are edited.
func goalConflictStrip(_ struct{}) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := appstate.Default
	viewAtom := uistate.UseGoalsView()
	goEarmarks := ui.UseEvent(Prevent(func() { viewAtom.Set(uistate.GoalsViewEarmarks) }))
	conflicts := computeGoalConflicts(app)
	if len(conflicts) == 0 {
		return Fragment()
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	var rows []ui.Node
	for _, c := range conflicts {
		over := c.ClaimMinor - c.BalMinor
		rows = append(rows, Div(css.Class("budget-issue-row"), Attr("data-testid", "goal-conflict-"+c.AccountID),
			Div(css.Class("budget-issue-main"),
				Span(css.Class("budget-issue-title"),
					uistate.T("goals.conflictTitle", strings.Join(c.GoalNames, ", "), c.AccountName)),
				Span(css.Class("budget-issue-body"),
					uistate.T("goals.conflictBody",
						fmtMoney(money.New(c.ClaimMinor, base)),
						fmtMoney(money.New(c.BalMinor, base)),
						fmtMoney(money.New(over, base)))),
			),
		))
	}
	return Div(css.Class("budget-issues-wrap"), Attr("data-testid", "goals-conflict-strip"),
		Div(css.Class("budget-issues-detail"),
			append([]ui.Node{}, rows...),
			Button(css.Class("btn btn-sm", tw.Mt2), Type("button"), Attr("data-testid", "goals-conflict-review"),
				Title(uistate.T("goals.conflictReviewTitle")), OnClick(goEarmarks), uistate.T("goals.conflictReview")),
		),
	)
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
