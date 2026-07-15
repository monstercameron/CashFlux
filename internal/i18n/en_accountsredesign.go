// SPDX-License-Identifier: MIT

package i18n

// accountsRedesignKeys holds English copy introduced by the accounts-page redesign:
// the per-row progressive-disclosure toggle and the clarified sweep-rules field
// labels. Kept in its own file (not en.go) so it doesn't touch the concurrent-WIP
// catalog.
var accountsRedesignKeys = Catalog{
	// Per-account row: a quiet disclosure that reveals the trend, flow, projection,
	// documents and notes so the resting row stays scannable.
	"accountsRedesign.detailsShow": "Details",
	"accountsRedesign.detailsHide": "Hide details",
	// %s = account name.
	"accountsRedesign.detailsAria": "More detail for %s",

	// Sweep-rules add form: sentence-case field labels above each control.
	"accountsRedesign.sweepKeepIn": "Keep in",
	"accountsRedesign.sweepMoveTo": "Move the extra to",
}

func init() {
	for k, v := range accountsRedesignKeys {
		english[k] = v
	}
}
