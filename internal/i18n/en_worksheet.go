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
	// Two strings rather than one signed number: "your statement is -$100.00 away
	// from this" makes a reader work out which way the gap runs before they can
	// start looking, and a bracketed negative makes it worse.
	"worksheet.residualHigher": "Your statement is %s higher than this. That much arrived without a transaction to explain it.",
	"worksheet.residualLower":  "Your statement is %s lower than this. That much left without a transaction to explain it.",
}

func init() {
	for k, v := range worksheetKeys {
		english[k] = v
	}
}
