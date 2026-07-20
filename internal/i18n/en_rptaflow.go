// SPDX-License-Identifier: MIT

package i18n

// rptaFlowKeys holds English strings for the 2026-07-19 Money-flow sankey fixes:
// disambiguating a category node whose name collides with the "Income" hub (the
// collision formed a From==To self-loop the layout drops, silently hiding e.g. a
// salary categorized under a category literally named "Income"), and the explicit
// deficit inflow shown when the period overspent. Merged via init so the shared
// en.go is never touched by this concurrent lane.
var rptaFlowKeys = Catalog{
	// A category node whose name equals the hub's label. %s = the category name;
	// e.g. "Income (category)".
	"rpta.nodeCatDisamb": "%s (category)",
	// The inflow ribbon that balances the hub when spending exceeded income.
	"rpta.nodeFromSavings": "Drawn from savings",
}

func init() {
	for k, v := range rptaFlowKeys {
		english[k] = v
	}
}
