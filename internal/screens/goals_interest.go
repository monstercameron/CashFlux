// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goalinterest"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// goals_interest.go renders the GL2 interest-aware ETA line on a financial goal
// card when a linked account carries an APY. The projection math is the tested
// internal/goalinterest package; this file only formats.

// linkedAPY returns the APY (percent) of the first linked account that carries a
// positive yield, and true when one exists. Accounts are the goal's linked list.
func linkedAPY(g domain.Goal, accounts []domain.Account) (float64, bool) {
	linked := g.LinkedAccountIDs()
	for _, id := range linked {
		for _, a := range accounts {
			if a.ID == id && a.APY > 0 {
				return a.APY, true
			}
		}
	}
	return 0, false
}

// goalInterestEtaLine renders the interest-aware ETA for a financial goal linked
// to an APY-bearing account, or Fragment() when there is no APY, no monthly
// contribution, or the goal is already complete. The line is explainable: it
// shows the contribution + rate, the projected months, and how much the interest
// itself adds (the contributions-vs-interest split).
func goalInterestEtaLine(g domain.Goal, accounts []domain.Account, now time.Time) ui.Node {
	apy, ok := linkedAPY(g, accounts)
	if !ok {
		return Fragment()
	}
	monthly, ok, err := goalsvc.MonthlyAssignment(g, now)
	if err != nil || !ok || monthly.Amount <= 0 {
		return Fragment()
	}
	cur := g.TargetAmount.Currency
	proj := goalinterest.Project(g.CurrentAmount.Amount, monthly.Amount, g.TargetAmount.Amount, apy)
	if !proj.Reached {
		return Span(css.Class("budget-sub"), Attr("data-testid", "goal-interest-eta-"+g.ID),
			uistate.T("goals.interestUnreached"))
	}
	if proj.Months <= 0 {
		return Fragment() // already at target
	}
	apyStr := fmt.Sprintf("%g%%", apy)
	eta := uistate.T("goals.interestEta",
		fmtMoney(monthly), apyStr, fmtMoney(g.TargetAmount), proj.Months)
	breakdown := uistate.T("goals.interestBreakdown", fmtMoney(money.New(proj.InterestMinor, cur)))
	return Span(css.Class("budget-sub"), Attr("data-testid", "goal-interest-eta-"+g.ID),
		Style(map[string]string{"color": "var(--up)"}),
		eta+" · "+breakdown)
}
