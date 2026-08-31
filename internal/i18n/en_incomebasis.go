// SPDX-License-Identifier: MIT

package i18n

// incomeBasisKeys holds the copy for the zero-based "by source" income basis: the
// select option that turns it on, and the income-source ledger (title, running-total
// caption, the held-aside tag, the empty state, and the uncategorized-income label).
// Merged via init so this file does not touch en.go.
var incomeBasisKeys = Catalog{
	"budgets.zbbBasisCategories":    "Choose income sources",
	"budgets.incomeSourcesTitle":    "Income sources",
	"budgets.incomeSourcesTotalCap": "Budgeting against",
	"budgets.incomeSourceHeldAside": "held aside",
	"budgets.incomeSourcesEmpty":    "No income categories yet. Categorize your deposits, then pick which ones fund your budget.",
	"budgets.incomeSourceUncat":     "Uncategorized income",
	"budgets.incomeSourceNoHistory": "No income last month",
	"budgets.incomeSelectAll":       "Include all",
	"budgets.incomeHoldAll":         "Hold all aside",
	"budgets.incomeSourcesCount":    "%d of %d included",
	"budgets.zbbAverageToggle":      "Average my last 3 months of income",
	"budgets.zbbAverageHint":        "Steadier than one month — good when income varies (freelance, commissions, tips).",
	// Allocation bar (zero-based hero) + the income-basis button and modal.
	"budgets.zbbUnassigned":        "Unassigned",
	"budgets.zbbOverAssignedShort": "Over-assigned",
	"budgets.zbbIncomeMarker":      "Your income",
	"budgets.zbbAllocRolled":       "incl. %s rolled over",
	"budgets.zbbAllocAria":         "How your income is allocated across expenses, savings, and what's left to assign",
	"budgets.zbbAllocAriaOver":     "Your income is fully assigned and over — the marker shows where income runs out; the fill past it is over-assigned",
	"budgets.basisButton":          "Budget income",
	"budgets.basisButtonTitle":     "Choose which income funds your budget",
	"budgets.basisModalTitle":      "Income to budget with",
	"budgets.basisModalHelp":       "Pick the income your budget is built on. Include a steady paycheck, hold aside irregular side income, or set a fixed monthly figure.",
}

func init() {
	for k, v := range incomeBasisKeys {
		english[k] = v
	}
}
