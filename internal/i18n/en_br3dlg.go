// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"br3dlg.raiseTitle":                "Raise %s limit",
		"br3dlg.coverTitle":                "Cover from another budget",
		"br3dlg.raisePlaceholder":          "0.00",
		"br3dlg.pickCover":                 "+%s (cover overspend)",
		"br3dlg.pickProjected":             "+%s (projected)",
		"br3dlg.rolloverNextCap":           "Next period's cap is the base limit plus carry.",
		"br3dlg.raisedToast":               "%s limit raised",
		"br3dlg.coveredFrom":               "Covered %s from %s",
		"budgets.actionCoverFromOtherMenu": "Cover from another budget",
	} {
		english[key] = value
	}
}
