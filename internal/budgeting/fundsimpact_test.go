// SPDX-License-Identifier: MIT

package budgeting

import "testing"

// No action on the budgets page moves money between real accounts — it changes
// the plan. Every one of them must be able to say so, which is the fact users
// most often assume the other way round.
func TestNoBudgetsPageActionMovesRealMoney(t *testing.T) {
	for i, a := range FundsActions {
		if a.MovesRealMoney() {
			t.Errorf("action %d claims to move an account balance; nothing on the budgets page does", i)
		}
	}
}

// Every action has to name at least one thing it changes. An empty description
// is how "Release unused funds" ended up meaning nothing in particular.
func TestEveryFundsActionNamesItsScope(t *testing.T) {
	for i, a := range FundsActions {
		if len(a.Scopes) == 0 {
			t.Errorf("action %d has no scope; every action must say what it touches", i)
		}
	}
}

// A one-time boost must NOT claim to change future periods, and a permanent one
// must. This is the distinction the top-up form's duration control exists for,
// and the one a user is most likely to get wrong.
func TestTopUpImpactFollowsTheDurationChoice(t *testing.T) {
	thisMonth := TopUpImpact(false, false)
	if !thisMonth.Touches(ScopeThisPeriod) || thisMonth.Touches(ScopeFuturePeriods) {
		t.Errorf("a this-period top-up = %v, want this period only", thisMonth.Scopes)
	}
	perm := TopUpImpact(true, false)
	if !perm.Touches(ScopeThisPeriod) || !perm.Touches(ScopeFuturePeriods) {
		t.Errorf("a permanent top-up = %v, want this period and future ones", perm.Scopes)
	}
}

// Funding a top-up from other budgets is what makes it a cross-budget action;
// unfunded, it comes from unassigned income and touches nothing else.
func TestTopUpImpactAddsOtherBudgetsOnlyWhenFunded(t *testing.T) {
	if TopUpImpact(false, false).Touches(ScopeOtherBudgets) {
		t.Error("an unfunded top-up claims to touch other budgets")
	}
	if !TopUpImpact(false, true).Touches(ScopeOtherBudgets) {
		t.Error("a funded top-up does not say it takes from other budgets")
	}
}

// TopUpImpact must not mutate the shared descriptions it derives from — a
// package-level slice appended to in place would leak "other budgets" onto every
// later caller.
func TestTopUpImpactDoesNotMutateTheSharedDescriptions(t *testing.T) {
	_ = TopUpImpact(false, true)
	_ = TopUpImpact(true, true)
	if ImpactTopUpThisPeriod.Touches(ScopeOtherBudgets) {
		t.Error("ImpactTopUpThisPeriod was mutated by a funded call")
	}
	if ImpactTopUpPermanent.Touches(ScopeOtherBudgets) {
		t.Error("ImpactTopUpPermanent was mutated by a funded call")
	}
}

// Deleting a budget IS reversible — Ctrl+Z restores it, verified in a browser.
// The confirmation used to say "This can't be undone", which is the C571 defect:
// a confirmation that overstates the risk teaches people to click through them.
func TestDeleteBudgetIsDescribedAsReversible(t *testing.T) {
	if !ImpactDeleteBudget.Reversible {
		t.Error("ImpactDeleteBudget.Reversible = false, but Ctrl+Z restores a deleted budget")
	}
	if !ImpactDeleteBudget.Destructive {
		t.Error("ImpactDeleteBudget.Destructive = false; removing a budget earns danger styling")
	}
}
