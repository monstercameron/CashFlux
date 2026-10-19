// SPDX-License-Identifier: MIT

package i18n

// gate1Keys contains the English copy moved into the catalog by the GATE1
// hardcoded-string cleanup.
var gate1Keys = Catalog{
	"sync.savedChangeInactive":            "This saved change is no longer in the active account and workspace.",
	"transactions.calFxMissingRate":       "Day totals are unavailable because an exchange rate is missing. Check your currency settings.",
	"transactions.calScheduledFilterNote": "Scheduled items follow account, category, and member filters. Other filters apply to transactions only.",
}

func init() {
	for key, value := range gate1Keys {
		english[key] = value
	}
}
