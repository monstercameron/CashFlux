// SPDX-License-Identifier: MIT

package i18n

import (
	"strings"
	"testing"
)

func TestLBL2TransactionGlossary(t *testing.T) {
	for _, tc := range []struct {
		key, want string
	}{
		{"transactions.searchPlaceholder", "Search merchant, details or tag"},
		{"transactions.payeeLabel", "Merchant or payee"},
		{"transactions.clearedStatus", "Bank status"},
		{"transactions.clearedAll", "Any"},
		{"transactions.cleared", "Cleared"},
		{"transactions.notCleared", "Not cleared"},
		{"transactions.confirmCategory", "Mark reviewed"},
		{"transactions.statusReviewed", "Reviewed"},
		{"transactions.chipFrom", "From %s"},
		{"transactions.chipTo", "Through %s"},
		{"transactions.deleteConfirm", `Delete "%s"? This can't be undone.`},
		{"transactions.deleteTransferConfirm", `Delete the transfer "%s"? This removes both sides — the money out and the matching money in. You can undo this.`},
		{"transactions.deleteTitle", "Delete transaction"},
		{"transactions.toggleClearedTitle", "Change bank status"},
		{"transactions.summary", "%s transactions · net %s"},
		{"transactions.scopeNet", "Net: %s"},
		{"transactions.badgeSuspectTransferTitle", "Looks like money moved between your accounts, but it still counts as income or spending. Choose the other account."},
		{"review.confirm", "Accept suggestions"},
		{"review.confirmN", "Accept %d suggestions"},
		{"review.bulkApplied", "Accepted %d suggestions."},
		{"review.pickHint", "Select transactions to accept suggestions."},
		{"review.tierReady", "Ready to accept"},
		{"sm10.hint", "About %s more than your usual %s at %s"},
		{"transactions.upcomingBadge", "Scheduled"},
		{"quickAdd.exclude", "Exclude from budgets and reports"},
		{"transactions.excludeLabel", "Exclude from budgets and reports"},
		{"transactions.excludeHint", "Still changes the account balance, but is left out of budgets and reports."},
		{"transactions.kebabExclude", "Exclude from budgets and reports"},
		{"transactions.bulkOpExcluded", "Left %d transactions out of budgets and reports"},
		{"rules.excludeFieldLabel", "Exclude from budgets and reports"},
		{"transactions.badgeSuspectTransfer", "Choose the other account"},
		{"transactions.badgeTransferTitle", "Money moved between your accounts — not income or spending."},
		{"r9f.tools", "Tools"},
		{"txnhistory.menuAction", "Change history"},
		{"r1b.catSuggest.menuAction", "Change category"},
		{"payeeClean.menuAction", "Edit merchant name"},
		{"transactions.createRule", "Create a rule from this transaction"},
		{"transactions.followUpTask", "Add follow-up task"},
		{"r9f.deleteTransaction", "Delete transaction"},
		{"txnkeys.review", "Mark the focused transaction reviewed"},
		{"txnkeys.reviewedToast", "Marked reviewed."},
	} {
		t.Run(tc.key, func(t *testing.T) {
			if got := english[tc.key]; got != tc.want {
				t.Fatalf("English[%q] = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}

func TestLBL2AccessibleNamesContainVisibleActions(t *testing.T) {
	for _, tc := range []struct {
		visible, accessible string
	}{
		{"Mark as cleared", "transactions.markClearedTitle"},
		{"Mark as not cleared", "transactions.markUnclearedTitle"},
		{"Delete selected", "transactions.deleteSelectedTitle"},
		{"Edit", "transactions.rowEditAria"},
	} {
		t.Run(tc.accessible, func(t *testing.T) {
			name := strings.ToLower(english[tc.accessible])
			if !strings.Contains(name, strings.ToLower(tc.visible)) {
				t.Errorf("accessible name %q does not contain visible label %q", english[tc.accessible], tc.visible)
			}
		})
	}
}
