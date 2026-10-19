// SPDX-License-Identifier: MIT

package i18n

// English strings for the TXW1 ledger accessibility improvements.
var txw1Keys = Catalog{
	"txw1.skipToTransactions": "Skip to transactions",
	"txw1.tableRegion":        "Transactions table",
	// %s values are the action, description, date, and formatted amount.
	"txw1.rowAction": "%s %s, %s, %s",
}

func init() {
	for key, value := range txw1Keys {
		english[key] = value
	}
}
