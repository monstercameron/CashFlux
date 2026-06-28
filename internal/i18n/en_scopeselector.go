// SPDX-License-Identifier: MIT

// Package i18n — ScopeSelector and multi-dimensional ScopeBanner strings (#444).
//
// Pattern: add new keys here only; init() merges them into the english catalog
// without touching en.go (concurrent WIP) or the existing en_scopebanner.go.
// Keys shared between the banner and the selector (scope.viewing, scope.shared)
// live here, co-located with the feature that owns them.
package i18n

var scopeSelectorKeys = Catalog{
	// Multi-dimensional banner prefix: "Viewing: Chase · Alice/Bob · Checking"
	"scope.viewing": "Viewing:",
	// "Shared" label for the group/household owner in banner and selector chips.
	"scope.shared": "Shared",
	// Scope selector section headings.
	"scope.institutions": "Institutions",
	"scope.owners":       "Owners",
	"scope.types":        "Types",
	"scope.accounts":     "Accounts",
	// Collapsible individual-account section toggle label.
	"scope.showAccounts": "Specific accounts",
	// Saved views panel.
	"scope.savedViews":                "Saved views",
	"scope.savedViews.select":         "Apply a saved view…",
	"scope.savedViews.save":           "Save current as…",
	"scope.savedViews.namePlaceholder": "View name",
	"scope.savedViews.confirm":        "Save",
	"scope.savedViews.cancel":         "Cancel",
	"scope.savedViews.delete":         "Delete view",
}

func init() {
	for k, v := range scopeSelectorKeys {
		english[k] = v
	}
}
