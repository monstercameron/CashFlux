// SPDX-License-Identifier: MIT

package i18n

var r1bKeys = Catalog{
	"r1b.catSuggest.menuAction":    "Change category",
	"r1b.categorySuggestionInline": " · %s?",
	"r1b.categorySuggestionAria":   "Use suggested category: %s",
	"r1b.autoFiledByRule":          "Filed by rule: %s",
	"transactions.linkTo":          "Link payment",
	"transactions.bankStatus":      "Bank status",
}

func init() {
	for k, v := range r1bKeys {
		english[k] = v
	}
}
