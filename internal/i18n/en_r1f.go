// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"txnkeys.title":         "Transaction keyboard shortcuts",
		"txnkeys.search":        "Focus transaction search",
		"txnkeys.next":          "Move to the next transaction",
		"txnkeys.previous":      "Move to the previous transaction",
		"txnkeys.select":        "Select or clear the focused transaction",
		"txnkeys.edit":          "Edit the focused transaction",
		"txnkeys.category":      "Edit the focused transaction and focus its category",
		"txnkeys.review":        "Confirm the focused transaction’s suggested category",
		"txnkeys.helpClose":     "Close this shortcut guide",
		"txnkeys.keySearch":     "/",
		"txnkeys.keyNext":       "j",
		"txnkeys.keyPrevious":   "k",
		"txnkeys.keySelect":     "x",
		"txnkeys.keyEdit":       "e or Enter",
		"txnkeys.keyCategory":   "c",
		"txnkeys.keyReview":     "r",
		"txnkeys.keyHelp":       "? or Escape",
		"txnkeys.close":         "Close",
		"txnkeys.reviewedToast": "Marked as reviewed.",
	} {
		english[key] = value
	}
}
