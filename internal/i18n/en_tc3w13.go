// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"tc3w13.spent":         "Spent",
		"tc3w13.income":        "Income",
		"tc3w13.net":           "Net",
		"tc3w13.futureSummary": "%s, %s posted, %s scheduled, %s due",
	} {
		english[key] = value
	}
}
