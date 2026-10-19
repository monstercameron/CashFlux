// SPDX-License-Identifier: MIT

package i18n

var budq5Keys = Catalog{
	"budgets.committedIncomplete":      "Some recurring amounts cannot be converted because an exchange rate is missing. Those budgets are not available to move money from.",
	"budgets.recommendationEvidence":   "Suggested limit: %s %s, based on %d recent months; %d unusual months excluded.",
	"budgets.recommendationIncomplete": "A limit suggestion is unavailable because an amount could not be converted or calculated.",
	"budgets.recommendationRaise":      "raise",
	"budgets.recommendationLower":      "lower",
	"budgets.recommendationKeep":       "keep",
	"budgets.rolloverChain":            "Period-by-period rollover: %s",
	"budgets.rolloverChainEntry":       "%s: base %s, carry-in %s, effective cap %s, spent %s, carry-out %s.",
}

func init() {
	for key, value := range budq5Keys {
		english[key] = value
	}
}
