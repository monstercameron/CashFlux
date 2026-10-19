// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

type fundsImpactReach string

const (
	fundsReachThisPeriod   fundsImpactReach = "this-period"
	fundsReachThisAndLater fundsImpactReach = "this-and-later"
	fundsReachThisAndNext  fundsImpactReach = "this-and-next"
	fundsReachFutureOnly   fundsImpactReach = "future-only"
	fundsReachPermanent    fundsImpactReach = "permanent"
	fundsReachDelete       fundsImpactReach = "delete"
)

type fundsImpactOption struct {
	reach                    fundsImpactReach
	movesFromSelectedBudgets bool
}

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
func fundsImpactLine(impact budgeting.FundsImpact, options ...fundsImpactOption) string {
	option := fundsImpactOption{reach: fundsReachPermanent}
	switch {
	case impact.Destructive && !impact.Touches(budgeting.ScopeOtherBudgets):
		option.reach = fundsReachDelete
	case !impact.Touches(budgeting.ScopeThisPeriod) && impact.Touches(budgeting.ScopeFuturePeriods):
		option.reach = fundsReachFutureOnly
	case impact.Touches(budgeting.ScopeThisPeriod) && !impact.Touches(budgeting.ScopeFuturePeriods):
		option.reach = fundsReachThisPeriod
	}
	if len(options) > 0 {
		option = options[0]
	}
	key := "budgets.impactReachThisPeriod"
	switch option.reach {
	case fundsReachThisAndLater:
		key = "budgets.impactReachThisAndLater"
	case fundsReachThisAndNext:
		key = "budgets.impactReachThisAndNext"
	case fundsReachFutureOnly:
		key = "budgets.impactReachFutureOnly"
	case fundsReachPermanent:
		key = "budgets.impactReachPermanent"
	case fundsReachDelete:
		key = "budgets.impactReachDelete"
	}
	if option.movesFromSelectedBudgets && impact.Touches(budgeting.ScopeOtherBudgets) {
		key += "Funded"
	}
	return uistate.T(key)
}
