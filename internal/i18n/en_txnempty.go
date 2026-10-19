// SPDX-License-Identifier: MIT

package i18n

// txnEmptyKeys contains copy for the transaction ledger's future-period empty
// state. It is kept separate from the shared English catalog and merged at init.
var txnEmptyKeys = Catalog{
	"transactions.noMatchFutureZero": "No transactions have been posted for this future period, and no scheduled items fall in it.",
	"transactions.noMatchFutureOne":  "No transactions have been posted for this future period. 1 scheduled item falls in it.",
	"transactions.noMatchFutureMany": "No transactions have been posted for this future period. %d scheduled items fall in it.",
	"transactions.noMatchFutureLink": "View Bills & recurring",
}

func init() {
	for k, v := range txnEmptyKeys {
		english[k] = v
	}
}
