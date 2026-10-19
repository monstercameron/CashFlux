// SPDX-License-Identifier: MIT

package i18n

// br2tbKeys contains the budgets toolbar copy and member-scope explanation for
// the BR2TB accessibility and consistency pass.
var br2tbKeys = Catalog{
	"bud2.memberScopeBR2TB":            "Showing %s's budgets: %d of %d include %s; amounts are household totals",
	"bud2.settingsAccessibleBR2TB":     "Settings — budget method, sorting and layout",
	"bud2.adjustAllBR2TB":              "Adjust every budget",
	"budgets.sortRiskBR2TB":            "Sort by: most at risk first",
	"budgets.ageMoneyOverYearWhyBR2TB": "Your spending is using income received more than a year ago.",
}

func init() {
	for key, value := range br2tbKeys {
		english[key] = value
	}
}
