// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"txnkeys.title":         "Keyboard shortcuts",
		"txnkeys.search":        "Search transactions",
		"txnkeys.next":          "Next transaction",
		"txnkeys.previous":      "Previous transaction",
		"txnkeys.select":        "Select transaction",
		"txnkeys.edit":          "Edit transaction",
		"txnkeys.category":      "Edit category",
		"txnkeys.review":        "Mark the focused transaction reviewed",
		"txnkeys.helpClose":     "Close this shortcut guide",
		"txnkeys.keySearch":     "/",
		"txnkeys.keyNext":       "j",
		"txnkeys.keyPrevious":   "k",
		"txnkeys.keySelect":     "x",
		"txnkeys.keyEdit":       "e or Enter",
		"txnkeys.keyCategory":   "c",
		"txnkeys.keyReview":     "r",
		"txnkeys.keyHelp":       "? or Escape",
		"txnkeys.close":         "Close shortcuts",
		"txnkeys.reviewedToast": "Marked reviewed.",
	} {
		english[key] = value
	}
}
