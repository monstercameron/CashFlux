// SPDX-License-Identifier: MIT

package i18n

// br3HeadKeys keeps the Budgets headline and its supporting copy in sentence case.
var br3HeadKeys = Catalog{
	"budgets.headlineSpentOf":              "spent of %s this month",
	"budgets.headlineSpentPast":            "spent of %s in %s",
	"budgets.headlineAttention":            "%d budgets need attention",
	"budgets.headlineMeterLabel":           "Spending compared with the monthly plan",
	"budgets.allocFitsIncome":              "Fits the available income",
	"budgets.ageMoneyOneLine":              "How long your cash can cover spending.",
	"budgets.allocCap":                     "Budgeted against income",
	"budgets.allocCapHist":                 "Budgeted against income that month",
	"budgets.recurring.title":              "Recurring bills",
	"budgets.recurring.allCategoriesTitle": "Recurring bills",
	"budgets.tmplOr":                       "or add one budget",
	"budgetMetrics.perDay":                 "Left per day",
	"budgetMetrics.daysLeft":               "Days left",
	"budgetMetrics.elapsed":                "Elapsed",
	"budgets.driversShow":                  "What's driving this",
	"budgets.smartNavLabel":                "Smart features",
	"budgets.br3.smartNavLabel":            "Smart features",
}

func init() {
	for key, value := range br3HeadKeys {
		english[key] = value
	}
}
