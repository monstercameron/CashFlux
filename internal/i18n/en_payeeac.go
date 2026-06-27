// SPDX-License-Identifier: MIT

package i18n

// payeeACKeys adds the C39/C46 payee-autocomplete strings to the English catalog
// via the init-merge pattern (kept separate from en.go which has concurrent WIP).
var payeeACKeys = Catalog{
	// Quick-Add payee field label and placeholder.
	"quickAdd.payee":            "Payee",
	"quickAdd.payeePlaceholder": "Who did you pay? (optional)",
}

func init() {
	for k, v := range payeeACKeys {
		english[k] = v
	}
}
