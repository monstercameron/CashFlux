// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// reservationNoticeProps adapts the same checked position to an inline Accounts
// warning or a Goals conflict panel. Callers retain their own navigation/actions.
type reservationNoticeProps struct {
	Position      goals.AccountReservation
	ShowGoalNames bool
}

func reservationNotice(props reservationNoticeProps) ui.Node {
	p := props.Position
	name := p.Account.Name
	if name == "" {
		name = uistate.T("integrity.unavailableAccount")
	}
	state, body := "breach", ""
	if !p.Known {
		state = "unknown"
		body = uistate.T("integrity.reservationUnavailable", name)
	} else if props.ShowGoalNames {
		body = uistate.T("goals.conflictBody", fmtMoney(p.Reserved), fmtMoney(p.Balance), fmtMoney(p.Over))
	} else {
		body = uistate.T("integrity.warnLine", fmtMoney(p.Balance), fmtMoney(p.Reserved), fmtMoney(p.Over))
	}
	if !props.ShowGoalNames {
		return Span(Attr("data-reservation-state", state), body)
	}
	var names []string
	for _, goal := range p.GoalRefs {
		names = append(names, goal.Name)
	}
	title := uistate.T("goals.conflictTitle", strings.Join(names, ", "), name)
	if !p.Known {
		title = uistate.T("integrity.reservationReview", name)
	}
	return Div(css.Class("budget-issue-main"), Attr("data-reservation-state", state),
		Span(css.Class("budget-issue-title"), title), Span(css.Class("budget-issue-body"), body))
}
