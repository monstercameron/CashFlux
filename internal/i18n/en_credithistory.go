// SPDX-License-Identifier: MIT

package i18n

var creditHistoryKeys = Catalog{
	"credit.historyBasis":            "Compared with your current credit limit using your current saved exchange rates.",
	"credit.historyUnavailable":      "Some balance records cannot be compared safely. Check their currencies, exchange rates, and the credit limit.",
	"credit.historyPointUnavailable": "Unavailable",
	"credit.historyAssumedCurrency":  "Older records without a currency use this account's current currency.",
	"credit.historyLatest":           "Latest %d of %d balance records.",
}

func init() {
	for key, value := range creditHistoryKeys {
		english[key] = value
	}
}
