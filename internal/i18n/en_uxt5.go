// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"uxt5.openFollowUp":        "1 open follow-up",
		"uxt5.openFollowUps":       "%d open follow-ups",
		"uxt5.receipt":             "1 receipt",
		"uxt5.receipts":            "%d receipts",
		"uxt5.hasNote":             "Has a note",
		"uxt5.autoAdded":           "Category added automatically",
		"uxt5.autoNeedsReview":     "Category added automatically — needs review",
		"uxt5.review":              "Needs review",
		"uxt5.cleared":             "Cleared",
		"uxt5.reconciled":          "Reconciled",
		"uxt5.info":                "Transaction details: %s",
		"uxt5.viewReceipt":         "View receipt",
		"uxt5.openFollowUpsAction": "Open follow-ups",
	} {
		english[key] = value
	}
}
