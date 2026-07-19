// SPDX-License-Identifier: MIT

package i18n

// budgetsHeroKeys holds English strings added by the 2026-07-19 /budgets "one
// answer" hero (B1): the page opens on the left-to-spend figure with a slim
// month-ledger bar and a single caption, replacing the three-cell status strip.
// Merged via init so the shared en.go is never touched by this concurrent lane.
var budgetsHeroKeys = Catalog{
	// The hero fig's label states tense + state; the value is only ever an amount
	// ("Jun 2026 · Over budget / $509.58" — never "Unspent / $509.58 over").
	"budgets.heroOverLabel": "Over budget",
	// The attention chip beside the hero number; clicking narrows the list.
	"budgets.heroAttn":    "%d need attention",
	"budgets.heroAttnOne": "1 needs attention",
	// Chip title: what a click does.
	"budgets.heroAttnTitle": "Show only the budgets that are over or near their limit",
	// Zero-based only: the still-unassigned pool, one small chip. %s = amount.
	"budgets.heroToAssign": "To assign: %s",
}

func init() {
	for k, v := range budgetsHeroKeys {
		english[k] = v
	}
}
