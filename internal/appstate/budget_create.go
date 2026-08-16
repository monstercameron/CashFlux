// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// CreateBudgetWithCategory saves a budget and — when newCat is non-nil — the
// brand-new category it watches as ONE operation (C584).
//
// The add-a-budget form used to write the two separately: PutCategory, then
// PutBudget. A budget write that failed after the category landed left an orphan
// category behind and a form the user had to re-submit, and nothing verified
// that either row could actually be read back before the dialog closed and
// reported success. Both halves belong to one user intent ("budget this new
// thing"), so they either both exist afterwards or neither does.
//
// On a failed budget write the just-created category is removed again, so a
// retry does not silently reuse a half-made category. The write is then READ
// BACK from the store: only a budget the store can return is a saved budget, and
// only then may the caller dismiss its dialog. Pure appstate — no syscall/js —
// so it is unit-tested on native Go.
//
// newCat must be a category that does not exist yet; pass nil when the budget
// attaches to an existing one.
func (a *App) CreateBudgetWithCategory(newCat *domain.Category, b domain.Budget) error {
	if newCat != nil {
		if err := a.PutCategory(*newCat); err != nil {
			return fmt.Errorf("appstate: create category %q for budget %q: %w", newCat.Name, b.Name, err)
		}
	}
	if err := a.PutBudget(b); err != nil {
		if newCat != nil {
			// Roll the category back so a failed add leaves nothing behind. A
			// rollback that itself fails is logged, not returned: the caller must
			// see the ORIGINAL failure, which is the one it can act on.
			if delErr := a.DeleteCategory(newCat.ID); delErr != nil {
				a.log.Error("rollback of category after failed budget write", "category", newCat.ID, "err", delErr)
			}
		}
		return fmt.Errorf("appstate: save budget %q: %w", b.Name, err)
	}
	if err := a.verifyBudgetSaved(newCat, b.ID); err != nil {
		return err
	}
	return nil
}

// verifyBudgetSaved re-reads the budget (and its new category) from the store.
// A write that reports success but cannot be read back is the exact shape of the
// defect C584 describes — a dialog that closes on a budget which is gone after a
// refresh — so the check is part of the write, not an optional extra.
func (a *App) verifyBudgetSaved(newCat *domain.Category, budgetID string) error {
	found := false
	for _, saved := range a.Budgets() {
		if saved.ID == budgetID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("appstate: budget %q is not readable after saving it", budgetID)
	}
	if newCat == nil {
		return nil
	}
	for _, c := range a.Categories() {
		if c.ID == newCat.ID {
			return nil
		}
	}
	return fmt.Errorf("appstate: category %q is not readable after saving it", newCat.ID)
}
