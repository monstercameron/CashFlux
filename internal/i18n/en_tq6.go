// SPDX-License-Identifier: MIT

package i18n

// txnTQ6Keys provides labels for one-sided transaction date filters.
var txnTQ6Keys = Catalog{
	"txnTQ6.fromDate":    "From %s onward",
	"txnTQ6.throughDate": "Through %s",
}

func init() {
	for key, value := range txnTQ6Keys {
		english[key] = value
	}
}
