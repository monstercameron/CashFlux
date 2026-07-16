// SPDX-License-Identifier: MIT

package i18n

// savedViewKeys holds English strings for the TX3 saved-views / watchlists
// feature (the "Views" toolbar affordance on /transactions, pinnable dashboard
// widgets, and per-view amount thresholds). Kept in its own file (not en.go) so
// this feature doesn't touch the concurrent-WIP catalog. Registered at init.
var savedViewKeys = Catalog{
	// Toolbar affordance.
	"savedViews.button":       "Views",
	"savedViews.title":        "Saved views",
	"savedViews.empty":        "No saved views yet. Filter the ledger, then save it here to reuse with one tap.",
	"savedViews.saveCurrent":  "Save current view…",
	"savedViews.saveDisabled": "Add at least one filter to save it as a view.",

	// Per-view row: %s = formatted total, %s = match-count phrase.
	"savedViews.rowSummary": "%s · %s",
	// Match count, correctly pluralized (%s across is not usable with %d).
	"savedViews.matchesOne":  "1 match",
	"savedViews.matchesMany": "%d matches",
	"savedViews.open":        "Open",
	"savedViews.eyebrow":     "Saved view",
	"savedViews.apply":       "Apply",
	"savedViews.applyAria":   "Apply the view “%s” to the ledger",
	"savedViews.pin":         "Pin to dashboard",
	"savedViews.pinned":      "“%s” is now on your dashboard.",
	"savedViews.rowMenu":     "More actions for “%s”",
	"savedViews.setAlert":    "Set amount alert…",
	"savedViews.editAlert":   "Edit amount alert…",
	"savedViews.delete":      "Delete view",
	"savedViews.deleted":     "Deleted “%s”.",

	// Save form.
	"savedViews.nameLabel":       "Name this view",
	"savedViews.namePlaceholder": "e.g. Amazon this month",
	"savedViews.save":            "Save view",
	"savedViews.cancel":          "Cancel",
	"savedViews.nameRequired":    "Give the view a name.",
	"savedViews.nameTaken":       "You already have a view with that name.",

	// Threshold / alert.
	"savedViews.thresholdLabel":       "Alert when the total reaches",
	"savedViews.thresholdPlaceholder": "Amount (leave blank for no alert)",
	"savedViews.thresholdSaved":       "Alert updated.",
	// %s = formatted total, %s = view name.
	"savedViews.thresholdNotice":  "“%s” has reached %s.",
	"savedViews.thresholdDismiss": "Dismiss",
}

func init() {
	for k, v := range savedViewKeys {
		english[k] = v
	}
}
