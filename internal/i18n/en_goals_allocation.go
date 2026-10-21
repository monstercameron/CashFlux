// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range (Catalog{
		"goals.allocationsSaved":               "Set-aside amounts saved.",
		"goals.allocTotalUnavailable":          "Total available amount could not be added safely.",
		"goals.allocAvailableUnknown":          "Available amount unavailable",
		"goals.allocExistingSourceUnavailable": "Existing reservation; this source cannot be changed",
		"goals.allocStoredAmounts":             "Stored reservation: %s",
		"goals.allocMissingAccount":            "Missing account (%s)",
		"goals.allocUnavailableExplanation":    "Some balances, reservations, or exchange rates are unavailable. Existing reservations stay unchanged unless you edit or remove them.",
		"goals.allocCurrencyChanged":           "The goal currency changed while you were editing. Review and enter the amounts again in the current currency.",
		"goals.allocAmountsOverflow":           "These amounts are too large to total safely. Lower the amounts and try again.",
		"goals.allocNoCapacity":                "The selected accounts have no available funds to split.",
		"goals.allocAmountInvalid":             "Enter a positive set-aside amount for %s, or uncheck that account.",
		"goals.allocSourceUnavailable":         "Available funds for %s could not be checked. Check its balance and exchange rates before changing an amount.",
		"goals.allocReviewAmounts":             "Review the set-aside amounts before saving.",
		"goals.allocAccountAmount":             "%s set-aside amount (%s)",
	}) {
		english[key] = value
	}
}
