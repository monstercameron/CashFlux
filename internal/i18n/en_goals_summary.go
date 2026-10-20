// SPDX-License-Identifier: MIT

package i18n

var goalsSummaryKeys = Catalog{
	// %s is the comma-separated list of currencies omitted from the summary.
	"goals.summaryExcludedCurrencies": "Some goal amounts are excluded from these totals because they could not be converted or added safely: %s.",
}

func init() {
	for key, value := range goalsSummaryKeys {
		english[key] = value
	}
}
