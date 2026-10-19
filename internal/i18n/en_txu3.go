// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"txu3.needAccount":     "Choose an account",
		"txu3.needAmount":      "Enter an amount",
		"txu3.needDescription": "Enter a description",
		"txu3.sentenceLabel":   "Describe it in a sentence",
		"txu3.needsReview":     "Needs review later",
		"txu3.needsReviewHelp": "Leave unchecked when this transaction is already reviewed.",
	} {
		english[key] = value
	}
}
