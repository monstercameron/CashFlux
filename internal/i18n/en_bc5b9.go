// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"bc5b9.budgetRowOf":       "of",
		"budgets.addedCount.one":  "1 budget",
		"budgets.addedCount.many": "%d budgets",
	} {
		english[key] = value
	}
}
