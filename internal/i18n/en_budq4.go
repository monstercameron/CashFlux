// SPDX-License-Identifier: MIT

package i18n

var budq4Keys = Catalog{
	"budgets.topupBtn":       "Raise limit…",
	"budgets.incomplete":     "Some figures are incomplete because a conversion or calculation is unavailable.",
	"budq4.adjustNormalized": "Comparable monthly plan: %s → %s",
}

func init() {
	for key, value := range budq4Keys {
		english[key] = value
	}
}
