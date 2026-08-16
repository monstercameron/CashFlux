// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// fundsImpactLine renders one budgets-page action's reach in the SAME words
// every other action on the page uses (C597).
//
// Release unused funds, Sweep leftovers, Delete budget, Top up, Cover and Adjust
// all have materially different effects, and each explained itself in its own
// vocabulary or not at all — so a user could not tell whether they were about to
// change one period or every period, one budget or several, the plan or the
// money. One sentence, assembled from one description, makes those comparable.
//
// The "no real money moves" clause is not filler: every action here changes a
// plan, and users routinely assume the opposite of anything with "funds" in its
// name.
func fundsImpactLine(impact budgeting.FundsImpact) string {
	var parts []string
	switch {
	case impact.Touches(budgeting.ScopeThisPeriod) && impact.Touches(budgeting.ScopeFuturePeriods):
		parts = append(parts, uistate.T("budgets.impactThisAndFuture"))
	case impact.Touches(budgeting.ScopeFuturePeriods):
		parts = append(parts, uistate.T("budgets.impactFutureOnly"))
	case impact.Touches(budgeting.ScopeThisPeriod):
		parts = append(parts, uistate.T("budgets.impactThisPeriodOnly"))
	}
	if impact.Touches(budgeting.ScopeOtherBudgets) {
		parts = append(parts, uistate.T("budgets.impactOtherBudgets"))
	}
	if !impact.MovesRealMoney() {
		parts = append(parts, uistate.T("budgets.impactNoRealMoney"))
	}
	if impact.Reversible {
		parts = append(parts, uistate.T("budgets.impactReversible"))
	}
	return strings.Join(parts, " ")
}
