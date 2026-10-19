// SPDX-License-Identifier: MIT

package i18n

import "testing"

func TestBR3DLGBudgetDialogCopyIsConsistent(t *testing.T) {
	for key, want := range map[string]string{
		"br3dlg.raiseTitle":                "Raise %s limit",
		"br3dlg.pickCover":                 "+%s (cover overspend)",
		"br3dlg.pickProjected":             "+%s (projected)",
		"br3dlg.rolloverNextCap":           "Next period's cap is the base limit plus carry.",
		"br3dlg.raisedToast":               "%s limit raised",
		"br3dlg.coverTitle":                "Cover from another budget",
		"budgets.actionCoverFromOtherMenu": "Cover from another budget",
	} {
		if got := english[key]; got != want {
			t.Errorf("English copy %q = %q, want %q", key, got, want)
		}
	}
	if got := english["br3dlg.raisePlaceholder"]; got == english["budgets.raiseAmount"] {
		t.Errorf("amount placeholder repeats its label: %q", got)
	}
}
