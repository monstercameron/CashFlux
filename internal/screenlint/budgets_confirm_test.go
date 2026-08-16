// SPDX-License-Identifier: MIT

package screenlint

import (
	"strings"
	"testing"
)

// TestBudgetConfirmationsDoNotClaimIrreversibility extends the C571 ratchet to
// the budgets surface (C597).
//
// "Delete the \"Travel\" budget? This can't be undone." was false: Ctrl+Z
// restores a deleted budget — verified in a browser before the copy was changed.
// Every money-moving action on that page is reversible, so any confirmation
// claiming otherwise is not being cautious, it is training people to click
// through confirmations. That is the same defect C571 fixed for duplicates, in a
// surface where the stakes read higher.
func TestBudgetConfirmationsDoNotClaimIrreversibility(t *testing.T) {
	for rel, text := range readInternal(t) {
		if !strings.HasPrefix(rel, "i18n/") {
			continue
		}
		for _, line := range strings.Split(text, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue // a comment may quote the retired copy
			}
			if !strings.Contains(line, "budgets.") || !strings.Contains(line, "onfirm") {
				continue
			}
			if strings.Contains(line, "can't be undone") || strings.Contains(line, "cannot be undone") {
				t.Errorf("%s: a budgets confirmation claims the action is irreversible:\n  %s\n"+
					"Ctrl+Z restores a deleted budget and every funds-moving action on the page "+
					"can be taken back, so the claim is false and it teaches users that "+
					"confirmations are noise (C597, extending C571).",
					rel, strings.TrimSpace(line))
			}
		}
	}
}

// TestEveryBudgetFundsActionExplainsItsReach keeps the six money-moving flows
// speaking one vocabulary.
//
// Release, Top up, Cover, Adjust all, Delete and Remove-recurring each used to
// explain themselves in their own words, or not at all, so a user could not tell
// which of them changed one period, every period, other budgets, or an account
// balance. fundsImpactLine is that one description; a flow that stops calling it
// has quietly gone back to inventing its own.
func TestEveryBudgetFundsActionExplainsItsReach(t *testing.T) {
	files := readInternal(t)
	want := map[string]string{
		"screens/budgets.go":           "the delete and remove-recurring confirmations",
		"screens/budgets_row.go":       "the release-unused confirmation",
		"screens/budgets_edit_form.go": "the top-up and cover forms",
		"screens/budgets_adjustall.go": "the adjust-all form",
	}
	for rel, what := range want {
		text, ok := files[rel]
		if !ok {
			t.Errorf("%s not found; %s should live there", rel, what)
			continue
		}
		if !strings.Contains(text, "fundsImpactLine(") {
			t.Errorf("%s no longer calls fundsImpactLine — %s must state their reach in the "+
				"same words as every other funds-moving action (C597).", rel, what)
		}
	}
}
