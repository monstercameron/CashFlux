// SPDX-License-Identifier: MIT

package i18n

// txImpactKeys holds English strings for the TX17 entry-time budget-impact
// caption shown live in Quick-Add. Kept out of en.go (concurrent-WIP file) and
// registered into the catalog at init time.
var txImpactKeys = Catalog{
	// Normal case: %s = amount left in the budget, %s = budget name, %s = safe-to-spend.
	"txImpact.leaves": "Leaves %s in %s this month · safe to spend %s",

	// Warning case: %s = budget name, %s = amount over the limit, %s = safe-to-spend.
	"txImpact.overBudget": "This puts %s over by %s · safe to spend %s",

	// No-budget case: %s = safe-to-spend.
	"txImpact.safeOnly": "Safe to spend %s this month",
}

func init() {
	for k, v := range txImpactKeys {
		english[k] = v
	}
}
