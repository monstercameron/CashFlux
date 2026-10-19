// SPDX-License-Identifier: MIT

package i18n

// v5rowKeys contains accessible labels and save confirmation copy for transaction rows.
var v5rowKeys = Catalog{
	"v5row.category":         "Category",
	"v5row.select":           "Select %s",
	"v5row.transactionSaved": "Transaction saved",
	"v5row.accountBalance":   "%s balance %s",
	"v5row.receipt":          "%d receipts, %s",
	"v5row.tag":              "Filter by tag %s, %s",
}

func init() {
	for key, value := range v5rowKeys {
		english[key] = value
	}
}
