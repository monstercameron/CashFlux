// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/goaltrajectory"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// GoalCompareProps drives the goal-vs-goal compare form rendered inside the
// shell-root flip modal (GoalCompareHost). OnDone closes it.
type GoalCompareProps struct {
	App    *appstate.App
	OnDone func()
}

// goalCompareStats is one goal's comparable figures, all derived from the same
// services the goal card itself uses — so the comparison can never disagree
// with the cards.
type goalCompareStats struct {
	target, covered, toGo money.Money
	monthly               money.Money
	hasMonthly            bool
	projected             time.Time
	reachable             bool
	deadline              time.Time
	priority              int
}

func computeGoalCompareStats(g domain.Goal, now time.Time) goalCompareStats {
	cur := g.TargetAmount.Currency
	covered := goalsvc.CoverageMinor(g)
	st := goalCompareStats{
		target:   g.TargetAmount,
		covered:  money.New(covered, cur),
		toGo:     money.New(max(g.TargetAmount.Amount-covered, 0), cur),
		deadline: g.TargetDate,
		priority: g.Priority,
	}
	if m, ok, _ := goalsvc.MonthlyAssignment(g, now); ok && m.Amount > 0 {
		st.monthly, st.hasMonthly = m, true
		res := goaltrajectory.Project(goaltrajectory.Input{
			CurrentMinor: covered, TargetMinor: g.TargetAmount.Amount,
			MonthlyMinor: m.Amount, Start: now, TargetDate: g.TargetDate,
		})
		st.projected, st.reachable = res.ProjectedDate, res.Reachable
	}
	return st
}

// GoalCompareForm is the goal-vs-goal comparison: pick two financial goals and
// read their figures side by side — target, saved + set aside, to go, monthly
// plan, projected landing, deadline, and priority.
func GoalCompareForm(props GoalCompareProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	aS := ui.UseState("")
	bS := ui.UseState("")

	var goals []domain.Goal
	if app != nil {
		for _, g := range app.Goals() {
			if !g.Archived && g.EffectiveKind().IsFinancial() && g.TargetAmount.Amount > 0 {
				goals = append(goals, g)
			}
		}
	}
	byID := func(id string) (domain.Goal, bool) {
		for _, g := range goals {
			if g.ID == id {
				return g, true
			}
		}
		return domain.Goal{}, false
	}
	opts := func(excludeID string) []uiw.SelectOption {
		out := []uiw.SelectOption{{Value: "", Label: uistate.T("goalcompare.pick")}}
		for _, g := range goals {
			if g.ID != excludeID {
				out = append(out, uiw.SelectOption{Value: g.ID, Label: g.Name})
			}
		}
		return out
	}

	now := time.Now()
	ga, okA := byID(aS.Get())
	gb, okB := byID(bS.Get())

	var table ui.Node = P(css.Class("muted"), Attr("data-testid", "goal-compare-empty"),
		uistate.T("goalcompare.empty"))
	if okA && okB {
		sa, sb := computeGoalCompareStats(ga, now), computeGoalCompareStats(gb, now)
		dash := uistate.T("goalcompare.none")
		fmtDate := func(t time.Time, ok bool) string {
			if !ok || t.IsZero() {
				return dash
			}
			return t.Format("Jan 2006")
		}
		fmtPriority := func(p int) string {
			if p <= 0 {
				return dash
			}
			return uistate.T(goalPriorityKey(p))
		}
		fmtMonthly := func(s goalCompareStats) string {
			if !s.hasMonthly {
				return dash
			}
			return fmtMoney(s.monthly)
		}
		row := func(labelKey, va, vb string) ui.Node {
			return Tr(Td(css.Class("t-caption"), uistate.T(labelKey)), Td(va), Td(vb))
		}
		table = Table(css.Class("goal-compare-table"), Attr("data-testid", "goal-compare-table"),
			Style(map[string]string{"width": "100%", "border-collapse": "collapse"}),
			Thead(Tr(Th(""), Th(ga.Name), Th(gb.Name))),
			Tbody(
				row("goalcompare.target", fmtMoney(sa.target), fmtMoney(sb.target)),
				row("goalcompare.covered", fmtMoney(sa.covered), fmtMoney(sb.covered)),
				row("goalcompare.toGo", fmtMoney(sa.toGo), fmtMoney(sb.toGo)),
				row("goalcompare.monthly", fmtMonthly(sa), fmtMonthly(sb)),
				row("goalcompare.projected", fmtDate(sa.projected, sa.reachable), fmtDate(sb.projected, sb.reachable)),
				row("goalcompare.deadline", fmtDate(sa.deadline, true), fmtDate(sb.deadline, true)),
				row("goalcompare.priority", fmtPriority(sa.priority), fmtPriority(sb.priority)),
			),
		)
	}

	return Div(css.Class("acct-edit-form"), Attr("data-testid", "goal-compare-form"),
		Div(css.Class("modal-scroll"),
			Div(Style(map[string]string{"display": "flex", "gap": "0.75rem", "flex-wrap": "wrap"}),
				Div(Style(map[string]string{"flex": "1 1 12rem"}),
					labeledField(uistate.T("goalcompare.goalA"),
						uiw.SelectInput(uiw.SelectInputProps{Options: opts(bS.Get()), Selected: aS.Get(),
							TestID: "goal-compare-a", OnChange: func(v string) { aS.Set(v) },
							AriaLabel: uistate.T("goalcompare.goalA")}))),
				Div(Style(map[string]string{"flex": "1 1 12rem"}),
					labeledField(uistate.T("goalcompare.goalB"),
						uiw.SelectInput(uiw.SelectInputProps{Options: opts(aS.Get()), Selected: bS.Get(),
							TestID: "goal-compare-b", OnChange: func(v string) { bS.Set(v) },
							AriaLabel: uistate.T("goalcompare.goalB")}))),
			),
			table,
		),
	)
}
