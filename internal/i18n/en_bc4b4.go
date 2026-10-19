// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"bc4b4.budgetCount.one":    "1 budget",
		"bc4b4.budgetCount.many":   "%d budgets",
		"bc4b4.categoryCount.one":  "1 category",
		"bc4b4.categoryCount.many": "%d categories",
		"bc4b4.tagCount.one":       "1 tag",
		"bc4b4.tagCount.many":      "%d tags",
		"bc4b4.sliderCount.one":    "1 slider",
		"bc4b4.sliderCount.many":   "%d sliders",
	} {
		english[key] = value
	}
}
