// SPDX-License-Identifier: MIT

package i18n

// catSuggestKeys holds the copy for the per-transaction "Categorize this" affordance
// (SM-2): the inline row chip that applies the local suggestion in one click, and the
// flip modal opened from the row kebab (suggestion + evidence + a category picker +
// the optional Smart+ ask). Merged via init so this file does not touch en.go.
var catSuggestKeys = Catalog{
	"catSuggest.title":      "Categorize this",
	"catSuggest.menuAction": "Categorize this…",
	// The inline row chip. %s = the suggested category name.
	"catSuggest.chipApply": "%s?",
	"catSuggest.chipAria":  "File this charge under %s",
	"catSuggest.chipHint":  "Suggested from your history. Click to file it here.",
	// The undoable toast after a one-click file. %s = the category it went into.
	"catSuggest.filedUndo": "Filed under %s.",
	// The batch form: %d = how many charges moved, %s = the category.
	"catSuggest.filedBatchUndo": "Filed %d charges under %s.",
	// Modal body.
	"catSuggest.txnLabel":       "Charge",
	"catSuggest.suggestedLabel": "Suggested category",
	"catSuggest.pickLabel":      "Category",
	"catSuggest.noSuggestion":   "Nothing local matches this charge yet — pick a category, or ask Smart+.",
	"catSuggest.useSuggestion":  "Use this",
	"catSuggest.save":           "File it",
	"catSuggest.needCategory":   "Pick a category.",
	// "Also apply to the other N charges from this merchant."
	"catSuggest.batchLabel": "Also file the other %d charges from this merchant",
	"catSuggest.suggestAI":  "Ask Smart+",
	"catSuggest.asking":     "Thinking…",
	// %s = whatever the model answered but the app could not match to a real category.
	"catSuggest.aiUnmatched": "Smart+ suggested \"%s\", which isn't one of your categories.",
	"catSuggest.aiSource":    "Suggested by Smart+.",
}

func init() {
	for k, v := range catSuggestKeys {
		english[k] = v
	}
}
