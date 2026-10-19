// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"r4loan1.groupTitle":          "Possible loan payments",
		"r4loan1.suggestionMessage":   "Looks like a payment to %s — record as a transfer?",
		"r4loan1.suggestionOnlyTitle": "A few payments may be loans",
		"r4loan1.suggestionOnlySub":   "Choose whether to record each one as a transfer.",
		"r4loan1.record":              "Record as transfer",
		"r4loan1.dismiss":             "Not a loan payment",
		"r4loan1.recordedUndo":        "Loan payment recorded as a transfer",
		"r4loan1.transferCategory":    "Transfer → %s",
	} {
		english[key] = value
	}
}
