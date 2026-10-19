// SPDX-License-Identifier: MIT

package i18n

var bc4s2aKeys = Catalog{
	"settings.budgetPeriodRekeyFailed": "Budget period data could not be moved to the new calendar. Your preference was not saved. Please try again.",
}

func init() {
	for key, value := range bc4s2aKeys {
		english[key] = value
	}
}
