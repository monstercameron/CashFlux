// SPDX-License-Identifier: MIT

package i18n

// merchantPanelKeys holds English strings for the merchant context panel (TX6)
// and receipt thumbnails (TX5) shown inside the transaction edit modal. Kept in
// its own file (not en.go) following the init-merge pattern so it doesn't collide
// with concurrent edits to the main catalog.
var merchantPanelKeys = Catalog{
	// Panel heading — the resolved merchant name is interpolated.
	"merchantPanel.title": "About %s",

	// Delta vs the merchant's typical charge. %s is the signed money delta.
	"merchantPanel.aboveUsual": "%s vs your usual",
	"merchantPanel.belowUsual": "%s vs your usual",
	"merchantPanel.atUsual":    "About your usual amount",

	// Visit frequency. %s is an ordinal ("3rd"); %d a count.
	"merchantPanel.visitThisWeek":   "%s visit this week",
	"merchantPanel.visitsThisMonth": "%d this month",

	// This-month spend vs a typical month. %[1]s spent, %[2]s typical.
	"merchantPanel.monthVsTypical": "%s this month · typical %s",

	// This-month spend with no prior-month baseline to compare against yet
	// (every charge on record is in the current calendar month).
	"merchantPanel.monthSpentOnly": "%s this month",

	// Accessible label for the tiny spend sparkline.
	"merchantPanel.sparklineAlt": "Recent charges at this merchant",

	// Receipt thumbnails (TX5).
	"merchantPanel.openReceipt": "Open receipt: %s",
}

func init() {
	for k, v := range merchantPanelKeys {
		english[k] = v
	}
}
