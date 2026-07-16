// SPDX-License-Identifier: MIT

package uistate

import "github.com/monstercameron/GoWebComponents/v4/state"

// BudgetAddSeed pre-fills the add-budget modal when it's opened from another surface:
// "Budget this" on an unbudgeted-spending chip (CategoryID + a suggested limit), or a
// future duplicate/copy affordance. Empty fields leave the form at its defaults.
type BudgetAddSeed struct {
	Name       string
	LimitMajor string // pre-formatted major-units string for the limit field ("450.00")
	CategoryID string // existing category to track ("" = the form's create-new default)
	Period     string // domain.Period value; "" = monthly default
}

// capturedBudgetAddSeed lets a click handler on another surface seed the next
// add-budget modal without calling UseAtom outside a render — the same captured-atom
// seam as TaskAddSeed.
var (
	capturedBudgetAddSeed state.Atom[BudgetAddSeed]
	budgetAddSeedCaptured bool
)

// UseBudgetAddSeed returns the atom holding the pre-fill for the next add-budget
// modal. AddHost reads it to seed BudgetAddForm; calling it in a render captures the
// atom for SetBudgetAddSeed.
func UseBudgetAddSeed() state.Atom[BudgetAddSeed] {
	a := state.UseAtom("budgets:addSeed", BudgetAddSeed{})
	capturedBudgetAddSeed = a
	budgetAddSeedCaptured = true
	return a
}

// SetBudgetAddSeed sets (or clears with the zero value) the pre-fill for the next
// add-budget modal. Safe from a click handler (uses the captured atom, not UseAtom).
func SetBudgetAddSeed(s BudgetAddSeed) {
	if budgetAddSeedCaptured {
		capturedBudgetAddSeed.Set(s)
	}
}
