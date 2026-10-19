// SPDX-License-Identifier: MIT

package i18n

// bq8Keys contains copy for safe funded budget top-ups.
var bq8Keys = Catalog{
	"budgets.coverCurrencyMismatch": "Choose source budgets that use the same currency as this budget.",
}

func init() {
	for key, value := range bq8Keys {
		english[key] = value
	}
}
