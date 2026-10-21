// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

type goalCardDetails struct {
	Interest, Trajectory, History ui.Node
}

// Keep expensive detail preparation behind one entry point after the compact
// return. The function value also lets actual-host tests observe preparations.
var prepareGoalCardDetails = buildGoalCardDetails

func buildGoalCardDetails(g domain.Goal, e goalsvc.Evaluation, accounts []domain.Account, rates currency.Rates, now time.Time) goalCardDetails {
	assignment, hasAssignment, assignmentErr := e.Assignment(g, rates)
	return goalCardDetails{
		Interest:   goalInterestEtaLine(g, e, accounts, now),
		Trajectory: goalTrajectoryNode(g, e, now),
		History:    goalContribHistoryNode(g, assignment, hasAssignment, assignmentErr, now),
	}
}

func goalNonFinancialSummary(g domain.Goal, e goalsvc.Evaluation, formatDate func(time.Time) string) ui.Node {
	var line string
	if e.Complete {
		line = uistate.T("goals.complete")
	} else if !g.TargetDate.IsZero() {
		line = uistate.T("goals.bySuffix", formatDate(g.TargetDate))
		line = strings.TrimPrefix(line, " · ")
	}
	if line == "" {
		return Fragment()
	}
	return Div(css.Class("budget-sub goal-sub"), Span(line))
}
