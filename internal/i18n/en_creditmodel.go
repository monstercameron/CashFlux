// SPDX-License-Identifier: MIT

package i18n

var creditModelKeys = Catalog{
	"credit.modelUnavailable":       "Unavailable",
	"credit.modelSourceUnavailable": "Credit data could not be loaded. Try again before using these figures.",
	"credit.modelBasis":             "All active household credit cards. Totals use %s and your current saved exchange rates, as of %s.",
	"credit.modelPartial":           "Some credit facts are unavailable. Check balances, currencies, and exchange rates before comparing all factors.",
	"credit.modelLimitSaveFailed":   "The limit could not be saved. Check the amount and reload if this account changed.",
}

func init() {
	for key, value := range creditModelKeys {
		english[key] = value
	}
}
