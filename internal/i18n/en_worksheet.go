// SPDX-License-Identifier: MIT

package i18n

// worksheetKeys holds the strings for the reconciliation worksheet (C690).
// Merged via init so this file never touches en.go.
//
// The dialog already states a difference; what it could not say is which term the
// difference is in. These label the terms in the words a person would use about
// their own account, not the words the ledger uses about itself.
var worksheetKeys = Catalog{
	"worksheet.heading":     "How this period adds up",
	"worksheet.opening":     "Balance at the start",
	"worksheet.moneyIn":     "Money in",
	"worksheet.moneyOut":    "Money out",
	"worksheet.transfers":   "Moved between your accounts",
	"worksheet.checkpoints": "Balance checkpoints",
	"worksheet.computed":    "Balance this adds up to",
	// Said only when there is something left over — the number to go looking for.
	"worksheet.residual": "Your statement is %s away from this. That difference is what is still unaccounted for.",
}

func init() {
	for k, v := range worksheetKeys {
		english[k] = v
	}
}
