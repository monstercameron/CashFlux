// SPDX-License-Identifier: MIT

package i18n

// reconcileSignKeys holds the strings the reconcile dialog needs to state its
// sign convention and to keep its Cancel button's promise (C683). Merged via
// init so this file never touches en.go.
var reconcileSignKeys = Catalog{
	// Said above the statement-balance field. The dialog used to ask for a number
	// without saying which sign it wanted, and then compared it against whichever
	// sign the account happened to store — so the same debt reconciled against
	// "500" on one account and "-500" on another.
	"accounts.reconSignHintAsset": "Enter the closing balance exactly as your statement prints it. Money in the account is a positive number.",
	"accounts.reconSignHintDebt":  "Enter the closing balance as it affects you: money you owe is a negative number, and a credit in your favour is positive. A statement showing $500.00 owed is −500.",
	// Cancel. It asks before discarding, because the rows ticked so far are real
	// work and the button that keeps them sits alongside.
	"accounts.reconCancelConfirm": "Undo the %d change(s) you made here and close? Nothing will be saved. Use “Save & finish later” to keep them.",
	"accounts.reconCancelDone":    "Put back %d change(s). Nothing was saved.",
	"accounts.reconCancelPartial": "%d change(s) could not be put back. Check the account before reconciling again.",
}

func init() {
	for k, v := range reconcileSignKeys {
		english[k] = v
	}
}
