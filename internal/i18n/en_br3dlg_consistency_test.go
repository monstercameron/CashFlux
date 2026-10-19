// SPDX-License-Identifier: MIT

package i18n

import "testing"

func TestBudgetCoverCopyDistinguishesOneBudgetFromCoverAll(t *testing.T) {
	const action = "Cover from another budget"
	if got := english["budgets.actionCoverFromOther"]; got != action {
		t.Errorf("full-card action = %q, want %q", got, action)
	}
	if got := english["budgets.actionCoverFromOtherMenu"]; got != action {
		t.Errorf("row-menu action = %q, want %q", got, action)
	}
	if got := english["br3dlg.coverBudgetTitle"]; got != "Cover %s from another budget" {
		t.Errorf("single-budget dialog title = %q, want a budget-named title", got)
	}
	if got := english["br3dlg.coverTitle"]; got != "Cover overages from other budgets" {
		t.Errorf("cover-all dialog title = %q, want the plural title", got)
	}
	if got := english["coverAll.title"]; got != "Cover overages from other budgets" {
		t.Errorf("Budgets cover-all title = %q, want the plural title", got)
	}
}
