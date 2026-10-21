// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

type goalCardDetails struct {
	Interest, Trajectory, History ui.Node
}

// Keep expensive detail preparation behind one entry point after the compact
// return. The function value also lets actual-host tests observe preparations.
var prepareGoalCardDetails = buildGoalCardDetails

func buildGoalCardDetails(g domain.Goal, e goalsvc.Evaluation, accounts []domain.Account, now time.Time) goalCardDetails {
	return goalCardDetails{
		Interest:   goalInterestEtaLine(g, e, accounts, now),
		Trajectory: goalTrajectoryNode(g, e, now),
		History:    goalContribHistoryNode(g, now),
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
