// SPDX-License-Identifier: MIT

package i18n

// nlSearchKeys holds English strings for TX2 — natural-language transaction
// search (the SMART-T3F Free tier). Kept in its own file (not en.go) so this
// change doesn't collide with concurrent edits to the main catalog.
var nlSearchKeys = Catalog{
	// The quiet "turn your words into filters" affordance under the search box.
	// %s is the previewed chip list (e.g. "Coffee · From 2026-05-01 · Over $20").
	"transactions.nlInterpret":     "Turn into filters: %s",
	"transactions.nlInterpretAria": "Apply these filters from your search: %s",

	// Flow (money direction) chips, from a "spent" / "received" clause.
	"transactions.chipFlowIn":  "Income only",
	"transactions.chipFlowOut": "Expenses only",
}

func init() {
	for k, v := range nlSearchKeys {
		english[k] = v
	}
}
