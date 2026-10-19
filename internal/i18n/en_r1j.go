// SPDX-License-Identifier: MIT

package i18n

// r1jKeys contains copy for the merchant-name cleanup confirmation toast.
var r1jKeys = Catalog{
	"payeeClean.savedToast": "Merchant name saved as %s.",
}

func init() {
	for key, value := range r1jKeys {
		english[key] = value
	}
}
