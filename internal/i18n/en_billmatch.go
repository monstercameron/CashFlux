// SPDX-License-Identifier: MIT

package i18n

// billMatchKeys holds the English strings for bill matching (TX9): the durable
// link between an expected recurring occurrence and the transaction that paid
// it. Defined in their own file and merged via init so this does not touch the
// shared en.go.
var billMatchKeys = Catalog{
	// Paid check shown on a recurring occurrence row once a real transaction has
	// been matched to it.
	"billmatch.paidBadge":      "Paid",
	"billmatch.paidBadgeTitle": "A matching transaction settled this occurrence.",
	// Variance chips: %s is the amount the payment differed from the expected bill.
	"billmatch.ranOver":  "ran %s over",
	"billmatch.ranUnder": "%s under",
	// Transaction row menu item / notice for unmatching a bill.
	"billmatch.unlink":       "Unlink bill",
	"billmatch.unlinkTitle":  "Release this transaction from the bill occurrence it was matched to",
	"billmatch.unlinkLogged": "Unlinked this transaction from its matched bill.",
}

func init() {
	for k, v := range billMatchKeys {
		english[k] = v
	}
}
