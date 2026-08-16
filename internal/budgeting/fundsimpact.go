// SPDX-License-Identifier: MIT

package budgeting

// FundsScope names one thing a money-moving action on the budgets page changes.
//
// C597: Release unused funds, Sweep leftovers, Delete budget, Top up, Cover and
// Adjust all have materially different effects — one period or every period, one
// budget or several, the plan or the money — and each explained itself (or did
// not) in its own words. A user could not tell from the dialog which of those an
// action was about to do.
//
// Naming the dimensions once, here, is what lets every one of those flows answer
// the same question in the same vocabulary, and what makes "does this move real
// money?" answerable at all — for every action on this page the answer is no,
// and none of them said so.
type FundsScope string

const (
	// ScopeThisPeriod: only the period on screen changes; the next one is untouched.
	ScopeThisPeriod FundsScope = "this-period"
	// ScopeFuturePeriods: the change carries forward — a limit, not a one-off boost.
	ScopeFuturePeriods FundsScope = "future-periods"
	// ScopeOtherBudgets: money comes out of, or goes into, budgets other than the
	// one being edited.
	ScopeOtherBudgets FundsScope = "other-budgets"
	// ScopeAccountBalance: real money moves between real accounts. Nothing on the
	// budgets page does this — which is precisely why it is worth stating.
	ScopeAccountBalance FundsScope = "account-balance"
)

// FundsImpact is the standard description of what one budgets-page action does.
type FundsImpact struct {
	// Scopes are the dimensions this action touches, in the order they should be
	// read: nearest-first (this period, then future, then other budgets).
	Scopes []FundsScope
	// Reversible is true when the action can be taken back — by Ctrl+Z, an
	// undoable toast, or simply doing the opposite. A confirmation that overstates
	// the risk teaches people that confirmations are noise (C571), so this is not
	// a matter of tone: it has to match what the handler actually does.
	Reversible bool
	// Destructive is true when the action removes something rather than moving a
	// number, and so earns danger styling and an explicit confirm.
	Destructive bool
}

// Touches reports whether the action changes the given dimension.
func (i FundsImpact) Touches(s FundsScope) bool {
	for _, got := range i.Scopes {
		if got == s {
			return true
		}
	}
	return false
}

// MovesRealMoney reports whether an account balance changes. Every budgets-page
// action answers false; the method exists so the UI states that rather than
// leaving the user to wonder whether "Release unused funds" emptied something.
func (i FundsImpact) MovesRealMoney() bool { return i.Touches(ScopeAccountBalance) }

// The page's actions, described once. Keeping them together is what keeps them
// consistent — and makes it obvious when a new action is added without one.
var (
	// ImpactTopUpThisPeriod raises a budget's cap for the period on screen only.
	ImpactTopUpThisPeriod = FundsImpact{Scopes: []FundsScope{ScopeThisPeriod}, Reversible: true}
	// ImpactTopUpPermanent raises the budget's limit from now on.
	ImpactTopUpPermanent = FundsImpact{Scopes: []FundsScope{ScopeThisPeriod, ScopeFuturePeriods}, Reversible: true}
	// ImpactRelease lowers this period's cap to what has been spent, returning the
	// remainder to the plan. Future periods keep the original limit.
	ImpactRelease = FundsImpact{Scopes: []FundsScope{ScopeThisPeriod}, Reversible: true}
	// ImpactCover moves limit from one budget to another, permanently.
	ImpactCover = FundsImpact{
		Scopes:     []FundsScope{ScopeThisPeriod, ScopeFuturePeriods, ScopeOtherBudgets},
		Reversible: true,
	}
	// ImpactAdjustAll scales every visible budget's limit from now on.
	ImpactAdjustAll = FundsImpact{
		Scopes:     []FundsScope{ScopeThisPeriod, ScopeFuturePeriods, ScopeOtherBudgets},
		Reversible: true,
	}
	// ImpactDeleteBudget removes a budget. Its transactions are untouched — they
	// simply stop being counted against a cap.
	ImpactDeleteBudget = FundsImpact{
		Scopes:      []FundsScope{ScopeThisPeriod, ScopeFuturePeriods},
		Reversible:  true, // Ctrl+Z restores it; the old copy claimed otherwise
		Destructive: true,
	}
	// ImpactRemoveRecurringCover stops a standing arrangement that would have
	// funded this budget in future periods.
	ImpactRemoveRecurringCover = FundsImpact{
		Scopes:      []FundsScope{ScopeFuturePeriods, ScopeOtherBudgets},
		Reversible:  true,
		Destructive: true,
	}
)

// FundsActions is every action described above, for the ratchet that checks none
// of them is left without a description.
var FundsActions = []FundsImpact{
	ImpactTopUpThisPeriod, ImpactTopUpPermanent, ImpactRelease,
	ImpactCover, ImpactAdjustAll, ImpactDeleteBudget, ImpactRemoveRecurringCover,
}

// TopUpImpact picks the top-up description for the chosen duration and whether
// the raise is funded from other budgets — the two choices the form offers.
func TopUpImpact(permanent, funded bool) FundsImpact {
	out := ImpactTopUpThisPeriod
	if permanent {
		out = ImpactTopUpPermanent
	}
	if funded {
		scopes := append([]FundsScope(nil), out.Scopes...)
		out.Scopes = append(scopes, ScopeOtherBudgets)
	}
	return out
}
