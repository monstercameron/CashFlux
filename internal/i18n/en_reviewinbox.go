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
	"review.chooseFirst":    "Choose a category first, then confirm.",

	// Why a SMART (local, non-LLM) suggestion was made — C515 returns the evidence
	// as data and it is phrased here. Never say "AI" for these: rules, history and
	// the merchant dictionary all run on-device and cost nothing.
	"review.whyRule":            "From a rule you wrote",
	"review.whyHistoryExact":    "You filed this charge here %d of %d times",
	"review.whyHistoryMerchant": "You filed %d of %d charges from this merchant here",
	"review.whyDictionary":      "%s is a well-known merchant",

	// Cross-link to /rules when ready-made rule suggestions exist (review #13/#18):
	// filing many of the queue in one place beats one-at-a-time triage.
	"review.rulesReadyOne":  "1 ready-made rule could file many of these",
	"review.rulesReadyMany": "%d ready-made rules could file many of these",
	"review.rulesReadyLink": "Review rules",

	"review.categorizedUndo":      "Categorized as %s.",
	"review.categorizedBatchUndo": "Categorized %d transactions as %s.",
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
