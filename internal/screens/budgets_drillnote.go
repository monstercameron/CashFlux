// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import "github.com/monstercameron/CashFlux/internal/uistate"

// budgetDrillChipKey names the ledger chip that says which budget a filtered
// view was opened from. It is a PRE-chip, like the member lens: it describes the
// origin of the view rather than being one of the page's own filters, and its ✕
// clears the whole drill scope instead of a single category.
const budgetDrillChipKey = "budgetdrill"

// budgetDrillChipKeyFor picks the chip's wording for what the budget's scope
// actually reaches (C585).
//
// The three cases are worth distinguishing because they answer different
// questions. A plain budget's ledger is exactly its category. A parent-category
// budget's ledger includes sub-categories the user never selected — the case
// that produced "$1,100.00 spent" beside "No matching transactions" when the
// drill only filtered the parent. A tag-tracking budget's ledger includes
// charges from categories that are not in the list at all.
func budgetDrillChipKeyFor(d uistate.BudgetDrill) string {
	switch {
	case d.Descendants && d.Tags:
		return "transactions.budgetDrillChipSubTags"
	case d.Descendants:
		return "transactions.budgetDrillChipSub"
	case d.Tags:
		return "transactions.budgetDrillChipTags"
	}
	return "transactions.budgetDrillChip"
}
