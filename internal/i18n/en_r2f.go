// SPDX-License-Identifier: MIT

package i18n

// r2fKeys contains plain-language labels for the Transactions filter sheet.
var r2fKeys = Catalog{
	"transactions.filterSearchAria":        "Search %s",
	"transactions.filterSearchPlaceholder": "Search %s",
	"transactions.filterSearchEmpty":       "No matches",
}

func init() {
	for key, value := range r2fKeys {
		english[key] = value
	}
}
