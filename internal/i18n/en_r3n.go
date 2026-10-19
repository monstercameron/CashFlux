// SPDX-License-Identifier: MIT

package i18n

// r3nKeys contains transaction count copy with correct singular and plural forms.
var r3nKeys = Catalog{
	"r3n.transactionCountOne":  "%s transaction",
	"r3n.transactionCountMany": "%s transactions",
}

func init() {
	for key, value := range r3nKeys {
		english[key] = value
	}
}
