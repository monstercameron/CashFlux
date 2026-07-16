// SPDX-License-Identifier: MIT

package i18n

// budgetPolishKeys holds v1.0 copy fixes for the Budgets screen: a singular
// variant of the over-budget banner (the plural read "1 budgets are over"), and
// a confirm prompt for the bulk 50/30/20 template create. Merged via init so
// this file does not touch en.go.
var budgetPolishKeys = Catalog{
	"budgets.overBannerOne":    "Overspent: 1 budget is over by %s total — review and cover the overspend.",
	"budgets.tmplConfirm":      "Create %s from the 50/30/20 template? You can edit or delete them afterward.",
	"budgets.tmplConfirmBtn":   "Create budgets",
	"budgets.tmplNothingToAdd": "Every 50/30/20 category already has a budget — nothing to add.",

	// Budget list name filter (shown when the list is long enough to be worth searching).
	"budgets.searchLabel":       "Filter budgets by name",
	"budgets.searchPlaceholder": "Filter budgets…",
	"budgets.searchNoMatch":     "No budgets match “%s”.",

	// Cross-category tag tracking on a budget.
	"budgets.tagsTracking":    "Also tracking tags:",
	"budgets.tagsFieldLabel":  "Track tags (cross-category)",
	"budgets.tagsFieldHint":   "Count any transaction with these tags, whatever its category. Comma-separated.",
	"budgets.tagsPlaceholder": "e.g. vacation, wedding",

	// The tracked-categories editor now edits categories AND tags — override the
	// category-only verbiage (base strings live in en.go; these win via init order).
	"budgets.catsAction":        "Edit tracking",
	"budgets.catsTitle":         "What this budget tracks",
	"budgets.editCatsOpen":      "Edit what this budget tracks…",
	"budgets.editCatsElsewhere": "Categories and tags are managed in their own editor.",
	"budgets.tracksCount":       "%s · %s", // "2 categories · 1 tag"

	// This-month selection metadata in the tracked-categories/tags editor.
	"budgets.tagsNoneThisMonth": "none this month",
	"budgets.trackMetaHint":     "Figures show this month so far — transactions · total.",

	// Linked follow-up to-dos in the budget card's side panel.
	"budgets.followUpsHead": "Follow-ups · %d open of %d",
	"budgets.followUpsMore": "+%d more in To-do",

	// The redesigned "What this budget tracks" editor: category + tag sections.
	"budgets.catsSection":  "Categories",
	"budgets.tagsSection":  "Tags (cross-category)",
	"budgets.tagsSearchPh": "Search tags, or type a new one…",
	"budgets.tagsNoneYet":  "No tags used yet — type one to add it.",
	"budgets.tagsAddNew":   "＋ Track a new tag: #%s",
	"budgets.tracksSave":   "Save",
}

func init() {
	for k, v := range budgetPolishKeys {
		english[k] = v
	}
}
