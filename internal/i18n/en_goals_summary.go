// SPDX-License-Identifier: MIT

package i18n

var goalsSummaryKeys = Catalog{
	"goals.toolbarLabel":             "Goal views, sorting, and actions",
	"goals.coverageUnavailableShort": "Coverage unavailable",
	"goals.coverageUnavailable":      "Coverage is unavailable because saved or reserved amounts could not be converted or added safely. Affected currencies: %s.",
	// %s is the comma-separated list of currencies omitted from the summary.
	"goals.summaryExcludedCurrencies":  "Some goal amounts are excluded from these totals because they could not be converted or added safely: %s.",
	"goals.fundExcludedCurrencies":     " Some fund targets are excluded because they could not be converted or added safely: %s.",
	"goals.earmarksIncomplete":         "Some accounts or reservations are excluded from these totals because their balances, amounts, or exchange rates could not be used safely.",
	"goals.earmarksExcludedCurrencies": " Affected currencies: %s.",
}

func init() {
	for key, value := range goalsSummaryKeys {
		english[key] = value
	}
}
