// SPDX-License-Identifier: MIT

package i18n

// accountsTilesKeys holds the English strings of the accounts page tiles that name a
// counted thing or join row metadata. Counted strings come as a singular/plural pair
// (read with uistate.TN, which passes the count as the first argument), so no caller
// pluralises a noun itself.
var accountsTilesKeys = Catalog{
	// Mark-all-updated confirm. %d = number of balances, %s = the accounts it touches.
	"accounts.markAllConfirmOne":  "Mark %d balance as updated? This updates: %s. You can undo it afterwards.",
	"accounts.markAllConfirmMany": "Mark %d balances as updated? This updates: %s. You can undo it afterwards.",
	// Mark-all-updated result toast. %d = number of balances.
	"accounts.markedUpdatedOne":  "Marked %d balance updated.",
	"accounts.markedUpdatedMany": "Marked %d balances updated.",

	// Type picker summary once several types are chosen. %d = number of types.
	"accounts.typesSelectedOne":  "%d type",
	"accounts.typesSelectedMany": "%d types",

	// Accessible name of the Filters button while filters are active. %d = number of filters.
	"accounts.filtersActiveAriaOne":  "Filters — %d filter active",
	"accounts.filtersActiveAriaMany": "Filters — %d filters active",

	// Account row subtitle. %s = type, %s = currency.
	"accounts.metaLine": "%s · %s",
	// Subtitle of a credit account that has a limit. %s = type, %s = currency, %d = percent of the limit used.
	"accounts.metaLineLimit": "%s · %s · %d%% of limit used",

	// Transfer form: the date is not a calendar date.
	"accounts.transferInvalidDate": "Enter the transfer date as a real date.",

	// Balance update toast for a debt whose sign had to be inferred. %s = account, %s = new balance.
	"accounts.balanceUpdatedGuessed": "Updated %s to %s. This debt has no opening balance, so whether you entered it as owed or as credit was inferred. Check that it reads right.",
}

func init() {
	for k, v := range accountsTilesKeys {
		english[k] = v
	}
}
