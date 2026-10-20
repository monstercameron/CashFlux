// SPDX-License-Identifier: MIT

package importdraft

import "strings"

// ExtractionErrors translates validation issues without showing provider errors
// or imported text as instructions. Unknown issues use the existing repair hint.
func ExtractionErrors(issues []string, translate Translate) string {
	keys := map[string]string{
		"Date is missing or invalid.":                  "importux.invalidDate",
		"Date is outside the supported range.":         "importux.dateRangeError",
		"Currency is missing.":                         "importux.missingCurrency",
		"Currency is unknown.":                         "importux.unknownCurrency",
		"Amount is missing or invalid.":                "importux.invalidAmount",
		"Amount is outside the supported range.":       "importux.amountRangeError",
		"Account does not match an available account.": "importux.unknownAccount",
	}
	var messages []string
	for _, issue := range issues {
		key := keys[issue]
		if key == "" {
			key = "imb.invalidRow"
		}
		messages = append(messages, translate(key))
	}
	if len(messages) == 0 {
		return translate("imb.invalidRow")
	}
	return strings.Join(messages, " ")
}
