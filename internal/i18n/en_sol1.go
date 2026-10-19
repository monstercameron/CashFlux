// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"sol1.transferUnavailable": "Couldn't use this account. Choose another account for this transfer.",
		"sol1.optional":            "Optional",
		"sol1.amountHint":          "Use Type to choose money in or money out.",
		"sol1.clearedHint":         "Matches your bank statement.",
		"sol1.useCategory":         "Use suggested category: %s",
		"sol1.bankDescription":     "Details shown on your bank statement.",
		"sol1.saveFailed":          "Couldn't save this transaction. Check the details and try again.",
		"sol1.deleteFailed":        "Couldn't delete this transaction. Try again.",
	} {
		english[key] = value
	}
}
