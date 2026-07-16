// SPDX-License-Identifier: MIT

package i18n

// reviewInboxKeys holds the strings for the transaction Review inbox (CG-S2) —
// the guided triage flow reached from the transactions toolbar. Merged via init
// so this file never touches en.go.
var reviewInboxKeys = Catalog{
	"review.title":  "Review transactions",
	"review.button": "Review inbox (%d)", // guided-triage entry; count = uncategorized + flagged (distinct from the filter chips below)

	"review.progress":            "%d of %d",
	"review.leftCount":           "%d left",
	"review.reasonUncategorized": "Uncategorized",
	"review.reasonFlagged":       "Flagged for review",
	"review.uncategorized":       "Uncategorized",

	"review.categoryLabel": "Category",
	"review.choose":        "Choose a category…",
	"review.suggested":     "Suggested: %s", // one-click apply (SMART, deterministic)
	"review.aiCategory":    "AI category",   // SMART+ (LLM picks an existing category)
	"review.aiThinking":    "Thinking…",
	"review.aiNoMatch":     "AI couldn't match a category — pick one above.",
	"review.alsoApply":     "Also apply to %d others from %s", // batch same-merchant

	"review.categorizeNext": "Categorize & next",
	"review.skip":           "Skip for now",
	"review.done":           "Done",

	"review.allDoneTitle":   "All caught up!",
	"review.allDoneClean":   "Nothing needs review right now.",
	"review.allDoneSkipped": "You skipped %d for now — everything else is reviewed.",
}

func init() {
	for k, v := range reviewInboxKeys {
		english[k] = v
	}
}
