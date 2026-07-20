// SPDX-License-Identifier: MIT

package i18n

// detail3Keys holds copy for the 2026-07-19 fine-detail polish (detail-lane 3) on
// /budgets: the density (compact-list) toggle surfaced directly on the page toolbar,
// so it no longer hides inside the Budget-settings popover. Merged via init so this
// file does not touch en.go.
var detail3Keys = Catalog{
	// The toolbar density toggle's accessible name (its visible affordance is the icon;
	// the tooltip reuses budgets.densityTitle).
	"budgets.densityToolbarAria": "Toggle compact list",
}

func init() {
	for k, v := range detail3Keys {
		english[k] = v
	}
}
