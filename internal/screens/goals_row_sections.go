// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/goalview"
	"github.com/monstercameron/CashFlux/internal/icon"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

type goalFiguresProps struct {
	Goal    domain.Goal
	Model   goalview.RowModel
	Compact bool
}

// GoalFigures presents the shared evaluated facts in compact or detailed form.
// It never recomputes coverage, required contributions, or growth projections.
func GoalFigures(props goalFiguresProps) ui.Node {
	pr := uistate.UsePrefs().Get()
	g, e := props.Goal, props.Model.Evaluation
	if e.Err != nil || e.Complete {
		return Fragment()
	}
	figs := []ui.Node{goalFig(uistate.T("goalsredesign.figToGo"), fmtMoney(e.Coverage.Remaining))}
	if e.HasMonthlyNeeded {
		figs = append(figs, goalFig(uistate.T("goals.figMonthlyNeeded"), fmtMoney(e.MonthlyNeeded)))
	}
	if !props.Compact && g.MonthlyContribution.Amount > 0 {
		figs = append(figs, goalFig(uistate.T("goals.figMonthlyPlan"), fmtMoney(g.MonthlyContribution)))
	}
	if !g.TargetDate.IsZero() {
		figs = append(figs, goalFig(uistate.T("goalsredesign.figTarget"), pr.FormatDate(g.TargetDate)))
	}
	if props.Model.Growth.Reachable && (!props.Compact || g.TargetDate.IsZero()) {
		figs = append(figs, goalFig(uistate.T("goalsredesign.figProjected", growthPctLabel(g.ExpectedReturnBips)), pr.FormatDate(props.Model.Growth.Date)))
	}
	id := "goal-figs-" + g.ID
	if props.Compact {
		id = "goal-compact-figs-" + g.ID
	}
	return Div(css.Class("goal-figs"), Attr("data-testid", id), figs)
}

func goalCoverageUnavailable(id string, e goalsvc.Evaluation) ui.Node {
	return P(css.Class("budget-sub", tw.TextWarn), Attr("data-testid", "goal-coverage-unavailable-"+id),
		uistate.T("goals.coverageUnavailable", strings.Join(e.Coverage.UnavailableCurrencies, ", ")))
}

type goalTaskPreviewProps struct {
	Goal     domain.Goal
	Summary  goalsvc.TaskSummary
	OnToggle func(string)
	OnAdd    ui.Handler
}

// GoalTaskPreview mounts only with the detailed card. Its bounded, preordered
// input avoids scanning or sorting the household task collection in each row.
func GoalTaskPreview(props goalTaskPreviewProps) ui.Node {
	g, s := props.Goal, props.Summary
	if s.Total == 0 && g.EffectiveKind() != domain.GoalKindChecklist {
		return Fragment()
	}
	var body ui.Node = P(css.Class("goal-todos-empty"), uistate.T("goals.noSteps"))
	if len(s.Preview) > 0 {
		items := make([]ui.Node, 0, len(s.Preview))
		for _, task := range s.Preview {
			items = append(items, ui.CreateElement(GoalTodoItem, goalTodoProps{Task: task, OnToggle: props.OnToggle}))
		}
		body = Div(css.Class("goal-todos-list"), items)
	}
	var more, add ui.Node = Fragment(), Fragment()
	if hidden := s.Total - len(s.Preview); hidden > 0 {
		more = P(css.Class("goal-todos-more"), Attr("data-testid", "goal-todos-more-"+g.ID), uistate.T("goals.stepsMore", hidden))
	}
	if !g.Archived && g.EffectiveKind() == domain.GoalKindChecklist {
		add = Button(css.Class("goal-todo-add"), Type("button"), Attr("data-testid", "goal-addstep-"+g.ID), OnClick(props.OnAdd),
			uiw.Icon(icon.PlusCircle, css.Class(tw.ShrinkO, tw.W35, tw.H35)), Span(uistate.T("goals.addStep")))
	}
	return Div(css.Class("goal-todos"), Attr("data-testid", "goal-todos-"+g.ID),
		Div(css.Class("goal-todos-head"), Span(css.Class("goal-todos-title"), uistate.T("goals.todosHead")), Span(css.Class("goal-todos-count"), fmt.Sprintf("%d/%d", s.Done, s.Total))), body, more, add)
}
