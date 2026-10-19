// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"bcs2.adjust.fromPeriod":  "From this period on",
		"bcs2.adjust.fromHint":    "Updates limits from the period you are viewing onward. Earlier periods keep their limits.",
		"bcs2.adjust.applyFrom":   "Apply from this period on",
		"bcs2.adjust.appliedFrom": "Adjusted %s by %s from this period on.",
		"bcs2.adjust.introFrom":   "Change budget limits from the period you are viewing onward.",
		"bcs2.copy.applied":       "Copied top-ups for %s into the next period.",
		"bcs2.copy.intro":         "Copy one-time funding into the next period. Money moved from other budgets or periods stays where it was moved. Older adjustments without a recorded source are kept in place.",
	} {
		english[key] = value
	}
}
