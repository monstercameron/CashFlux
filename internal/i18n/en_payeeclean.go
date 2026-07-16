// SPDX-License-Identifier: MIT

package i18n

// payeeCleanKeys holds the copy for the per-transaction payee-cleanup flip modal (SM-1)
// opened from a transaction row's kebab. Merged via init so this file does not touch en.go.
var payeeCleanKeys = Catalog{
	"payeeClean.title":      "Clean up merchant name",
	"payeeClean.menuAction": "Clean up name…",
	"payeeClean.rawLabel":   "As it appears on the transaction",
	"payeeClean.nameLabel":  "Clean name",
	"payeeClean.scopeLabel": "Apply to",
	// %d = how many transactions share this raw name.
	"payeeClean.scopeAll":      "All %d charges",
	"payeeClean.scopeThis":     "This transaction",
	"payeeClean.scopeAllHint":  "Maps this name everywhere — past and future imports — as a saved rule (also editable under Rules).",
	"payeeClean.scopeThisHint": "Renames only this one transaction; other charges keep their current name.",
	"payeeClean.suggestAI":     "Suggest with AI",
	"payeeClean.suggesting":    "Thinking…",
	"payeeClean.save":          "Save name",
	"payeeClean.needName":      "Enter a name.",
	// Rename history: the lineage of past clean names for this merchant.
	"payeeClean.historyLabel":    "Rename history",
	"payeeClean.historyOriginal": "Original",
	"payeeClean.historyCurrent":  "Now",
}

func init() {
	for k, v := range payeeCleanKeys {
		english[k] = v
	}
}
