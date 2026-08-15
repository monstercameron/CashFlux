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
	// Shared category picker (C499) — one implementation replacing four inline
	// quick-adds, and the only one that can create a SUB-category.
	"catpick.title":             "Choose a category",
	"catpick.searchPlaceholder": "Search categories, or type a new name",
	"catpick.noMatch":           "No category matches “%s”. Make it below.",
	"catpick.makeTitle":         "Make “%s”",
	"catpick.topLevel":          "as a top-level category",
	"catpick.inside":            "inside %s",
	"catpick.parentLabel":       "Where the new category goes",
	"catpick.create":            "Create",
	"catpick.close":             "Close",
	"catpick.newOption":         "+ New category or sub-category…",

	// Dual-mode review surface (C500–C512).
	"review.modeLabel":          "Review mode",
	"review.modeSingle":         "One at a time",
	"review.modeBulk":           "Bulk",
	"review.collapseNote":       "Charges are grouped by merchant and sorted by how confident the matcher is. Confirm a whole merchant at once, or open one to change individual charges.",
	"review.charges":            "charges",
	"review.decisions":          "decisions",
	"review.tierReady":          "Ready to confirm",
	"review.tierLook":           "Worth a look",
	"review.tierNone":           "No suggestion yet",
	"review.tierCount":          "%s · %s",
	"review.selectAll":          "Select all %d",
	"review.chargeCount":        "%s",
	"review.oneCharge":          "1 charge",
	"review.nothingPicked":      "Nothing selected",
	"review.pickHint":           "Pick a merchant, or confirm a whole tier at once.",
	"review.selCount":           "%s · %s",
	"review.undoStays":          "%s · undo stays available",
	"review.confirmN":           "Confirm %d",
	"review.confirm":            "Confirm",
	"review.clear":              "Clear",
	"review.makeRule":           "Always do this — make a rule",
	"review.ruleMade":           "Rule created for %s — future charges file themselves.",
	"review.snooze":             "Snooze",
	"review.dismiss":            "Not a problem",
	"review.notCategory":        "It's a transfer, not a category",
	"review.splitNeeded":        "This charge is split across categories and the parts don't add up. Fix the split rather than assigning a category.",
	"review.emptyTitle":         "Nothing needs a look",
	"review.emptySub":           "Every charge is categorized. New imports will show up here.",
	"review.showOther":          "Show the other %d",
	"review.keyboardHintBulk":   "j / k move · space picks · Enter confirms · 1 for one at a time",
	"review.keyboardHintSingle": "j / k move · Enter confirms · s snoozes · d dismisses · b for bulk",
	"review.bulkApplied":        "Categorized %d charges.",

	// SMART+ scan strip (C504/C509) — scope and cost stated BEFORE the button.
	"review.scanTitle":    "Let Smart+ suggest categories",
	"review.scanSub":      "It reads %d charges at a time and can only pick categories you already have. Costs about %s on your own key.",
	"review.scanBtn":      "Scan %d charges",
	"review.scanNotNow":   "Not now",
	"review.scanning":     "Reading %d charges…",
	"review.scanningSub":  "Only the payee, amount and your category names are sent.",
	"review.scanDone":     "Smart+ filled %d of the %d gaps your rules left",
	"review.scanDoneSub":  "It skipped %d rather than guess. This scan cost %s.",
	"review.scanMore":     "Scan the next %d",
	"review.useSuggested": "Use these suggestions",
	"review.filledN":      "Filled %d categories from Smart+",
	"review.needsYou":     "%d merchants still need you.",
	"review.legendHigh":   "Sure enough to confirm in bulk",
	"review.legendMid":    "Suggested, but check it",
	"review.legendNone":   "Skipped rather than guess",

	// Single-mode context band (C503).
	"review.bandTitle":    "What else this charge is tied to",
	"review.sibTitle":     "%d more charges from this merchant are waiting",
	"review.linkOrder":    "Part of one order, split across %d charges",
	"review.linkRefund":   "Paired with a refund",
	"review.dupeTitle":    "Looks like %d copies of the same charge",
	"review.typicalTitle": "Bigger than usual for this merchant",
	"review.typicalSub":   "typical charge is %s · this one is %s above",
	"review.reasonSplit":  "Split doesn't add up",

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
	"review.skip":                 "Skip for now",
	"review.done":                 "Done",

	"review.allDoneTitle":   "All caught up!",
	"review.allDoneClean":   "Nothing needs review right now.",
	"review.allDoneSkipped": "You skipped %d for now — everything else is reviewed.",
}

func init() {
	for k, v := range reviewInboxKeys {
		english[k] = v
	}
}
