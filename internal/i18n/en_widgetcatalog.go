// SPDX-License-Identifier: MIT

package i18n

import "maps"

// widgetCatalogKeys hold the widget catalog's labels (C362).
//
// internal/widgetcatalog is pure — it names every widget, column, chart and
// preset without any language context, so it carried its copy as finished
// English and the language setting could not reach the widget designer at all.
// It carries key + English side by side now, and the UI resolves at render time
// through the package's Localized helpers. Converted byte-for-byte; the rendered
// English is unchanged.
var widgetCatalogKeys = Catalog{
	"wcat.blockThisMonth":         "This month",
	"wcat.colAccount":             "Account",
	"wcat.colAmount":              "Amount",
	"wcat.colBalance":             "Balance",
	"wcat.colCategory":            "Category",
	"wcat.colDate":                "Date",
	"wcat.colDescription":         "Description",
	"wcat.colDueDate":             "Due date",
	"wcat.colName":                "Name",
	"wcat.colPayee":               "Payee",
	"wcat.colShare":               "Share",
	"wcat.colSource":              "Source",
	"wcat.colUsedPct":             "Used %",
	"wcat.collAccountBalances":    "Account balances",
	"wcat.collAllTransactions":    "All transactions",
	"wcat.collBudgetStatus":       "Budget status",
	"wcat.collRecentTransactions": "Recent transactions",
	"wcat.collSpendingByCategory": "Spending by category",
	"wcat.collUpcomingBills":      "Upcoming bills",
	"wcat.fmtArrow":               "Up/down arrow",
	"wcat.fmtCountNoun":           "Count + noun",
	"wcat.fmtMoney":               "Money",
	"wcat.fmtNumber":              "Number",
	"wcat.fmtPercent":             "Percent",
	"wcat.fmtSigned":              "Signed (+/−)",
	"wcat.fmtSignedMoney":         "Signed money (+/-)",
	"wcat.kindChart":              "Chart",
	"wcat.kindCompound":           "Custom layout",
	"wcat.kindFigure":             "Single figure",
	"wcat.kindList":               "List",
	"wcat.linkAllAccounts":        "View all accounts",
	"wcat.linkAllBills":           "View all bills",
	"wcat.linkAllBudgets":         "View all budgets",
	"wcat.linkAllTransactions":    "View all transactions",
	"wcat.linkSpendingReports":    "Open spending reports",
	"wcat.listCap":                "Show top rows",
	"wcat.listPage":               "Page through",
	"wcat.listScroll":             "Scroll all",
	"wcat.oDivider":               "Divider",
	"wcat.oEmbeddedData":          "Embedded data",
	"wcat.oFigureAMetric":         "Figure (a metric)",
	"wcat.oIcon":                  "Icon",
	"wcat.oSpacer":                "Spacer",
	"wcat.oTextCaption":           "Text / caption",
	"wcat.serCashFlow":            "Cash flow by month",
	"wcat.serNetWorth":            "Net worth over time",
	"wcat.sortAZ":                 "A → Z",
	"wcat.sortHighLow":            "High → Low",
	"wcat.sortLowHigh":            "Low → High",
	"wcat.sortZA":                 "Z → A",
	"wcat.srcCollection":          "Breakdown",
	"wcat.srcSeries":              "Trend over time",
	"wcat.startIncomeVsSpending":  "Income vs spending",
	"wcat.startNetWorth":          "Net worth",
	"wcat.startNetWorthTrend":     "Net worth trend",
	"wcat.startRecentActivity":    "Recent activity",
	"wcat.startSavingsRate":       "Savings rate",
	"wcat.startSpendingBreakdown": "Spending breakdown",
	"wcat.trFilter":               "Filter rows",
	"wcat.trLimit":                "Limit rows",
	"wcat.trSort":                 "Sort by column",
}

func init() {
	maps.Copy(english, widgetCatalogKeys)
}
