// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range (Catalog{
		"networth.unavailable":               "Net worth is unavailable because a balance or conversion could not be calculated safely.",
		"networth.partial":                   "Partial total · excludes %s without exchange rates: %s.",
		"networth.byChoice":                  "Excludes %s by choice.",
		"nws.factWindowStatus":               "Window details",
		"nws.windowUnavailable":              "Unavailable",
		"insights.affordNetWorthUnavailable": "I can't assess affordability while net worth is incomplete or unavailable. Check account balances and exchange rates first.",
	}) {
		english[key] = value
	}
}
