// SPDX-License-Identifier: MIT

package i18n

import "testing"

func TestSOL1TransactionFormGlossary(t *testing.T) {
	for _, tc := range []struct{ key, want string }{
		{"quickAdd.expense", "Money out"}, {"quickAdd.income", "Money in"}, {"quickAdd.transfer", "Transfer"},
		{"quickAdd.payee", "Merchant or payee"}, {"transactions.payeeLabel", "Merchant or payee"},
		{"quickAdd.description", "Bank description"}, {"transactions.descPlaceholder", "Bank description"},
		{"quickAdd.cleared", "Cleared"}, {"txnwidget.clearedLabel", "Cleared"},
		{"quickAdd.exclude", "Exclude from budgets and reports"}, {"transactions.excludeLabel", "Exclude from budgets and reports"},
		{"quickAdd.member", "Household member"}, {"transactions.whoLabel", "Household member"},
		{"r9f.deleteTransaction", "Delete transaction"}, {"sol1.optional", "Optional"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			if got := english[tc.key]; got != tc.want {
				t.Fatalf("English = %q, want %q", got, tc.want)
			}
		})
	}
}
