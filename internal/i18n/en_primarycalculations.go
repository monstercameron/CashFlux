// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range (Catalog{
		"calculation.unavailable":       "Unavailable",
		"dashboard.forecastUnavailable": "Forecast totals could not be calculated. Check your account balances and currency settings.",
		"dashboard.forecastExcluded":    "Some cash accounts were excluded because an exchange rate is missing.",
		"rpta.positionUnavailable":      "Household position could not be calculated. Check your account balances, debt details and currency settings.",
		"rpta.activityIncomplete":       "Some report figures could not be calculated. Check your account balances and currency settings.",
		"reports.controls":              "Report controls",
	}) {
		english[key] = value
	}
}
