// SPDX-License-Identifier: MIT

package i18n

// dupeScopeKeys holds the strings for account-scoped duplicate review (C688).
// Merged via init so this file never touches en.go.
var dupeScopeKeys = Catalog{
	// A last-line guard, so it explains rather than just refusing. Reached only if
	// a group was assembled outside the normal grouping rule.
	"duplicates.crossAccountRefused": "These entries are on different accounts, so they are two separate payments rather than one entered twice. Nothing was merged.",
	// Shown on each group, because "same day, same amount, same description" is
	// only a duplicate WITHIN one account, and the reader should see which.
	"duplicates.onAccount": "on %s",
}

func init() {
	for k, v := range dupeScopeKeys {
		english[k] = v
	}
}
