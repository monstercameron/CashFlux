// SPDX-License-Identifier: MIT

package i18n

// qpassEKeys holds English copy for the 2026-07-19 v1.2.7 review lane-E
// (responsive) fixes. Only the budget "Plan the year" annual grid needs new copy:
// a top-anchored scroll cue that signals the wide 12-month matrix continues to the
// right, so a narrow (expanded-sidebar) pane no longer hides the later months
// behind only a bottom scrollbar. The transactions condensed-ledger tier and the
// To-do calendar fit are pure layout and add no copy.
//
// Kept in its own file and merged via init so the shared en.go is never touched
// under concurrent work.
var qpassEKeys = Catalog{
	// Sits just above the annual-grid scroll frame; hidden once the pane is wide
	// enough to show the whole year. The trailing arrow points to the hidden months.
	"budgets.annualGridScrollCue": "Scroll sideways for the full year →",
}

func init() {
	for k, v := range qpassEKeys {
		english[k] = v
	}
}
