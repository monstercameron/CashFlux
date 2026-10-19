// SPDX-License-Identifier: MIT

package i18n

var tv7dKeys = Catalog{
	"review.tierCountPlain":          "%s, %s",
	"review.selectionCountPlain":     "%s, %s",
	"review.undoAvailable":           "Undo stays available",
	"review.keyboardHintBulkPlain":   "j / k moves, Space selects, Enter marks reviewed, 1 for one at a time",
	"review.keyboardHintSinglePlain": "j / k moves, Enter marks reviewed, s snoozes, d dismisses, b for several at once",
	"transactions.newCategoryOption": "New category…",
}

func init() {
	for key, value := range tv7dKeys {
		english[key] = value
	}
}
