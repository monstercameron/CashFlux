// SPDX-License-Identifier: MIT

package i18n

// netWorthRedesignKeys holds the English strings for the from-scratch /networth
// balance-sheet surface: the Glance | Detail views, THE BRIDGE, TWO SIDES, the
// takeaway, and the interpreted ratios. The older `nw.*` keys are still used for
// the figures both designs share (buckets, the "as of" line, the metrics
// toggle); everything new lives under `nws.*`. Merged via init so this file does
// not touch en.go — the en_networthsurface.go pattern.
var netWorthRedesignKeys = Catalog{
	// ── Views and window
	"nws.viewGlance":      "Glance",
	"nws.viewDetail":      "Detail",
	"nws.viewGlanceTitle": "The whole picture in one screen",
	"nws.viewDetailTitle": "The full balance sheet, section by section",
	"nws.viewAria":        "How much of the balance sheet to show",
	"nws.windowLabel":     "Period",
	"nws.winMonth":        "This month",
	"nws.winMonths":       "%d months",
	"nws.agoMonth":        "at the start of this month",
	"nws.agoMonths":       "%d months ago",
	"nws.agoYears":        "two years ago",
	"nws.deltaOver":       "over %s",
	"nws.deltaTitle":      "How your net worth has moved across %s",

	// ── THE BRIDGE
	"nws.bridgeTitle": "What moved it",
	"nws.bridgeNote":  "Every step between where you stood %s and where you stand now. The steps add up exactly — including the part we can't attribute.",
	"nws.bridgeAria":  "Waterfall chart decomposing the change in net worth from %s to %s.",
	"nws.bridgeFloor": "Bars are measured from %s, not from zero, so the steps stay readable.",
	"nws.legStart":    "Started at",
	"nws.legEnd":      "Now",

	"nws.legMoneyKept":       "Money kept",
	"nws.legMarket":          "Market movement",
	"nws.legDebtPaid":        "Debt paid down",
	"nws.legNewDebt":         "New debt",
	"nws.legRevaluation":     "Revalued",
	"nws.legResidual":        "Unexplained",
	"nws.legMoneyKeptWhat":   "What came in minus what went out, across your everyday and savings accounts.",
	"nws.legMarketWhat":      "Value your investment, retirement and crypto accounts gained or lost on their own.",
	"nws.legDebtPaidWhat":    "Payments that reduced what you owe.",
	"nws.legNewDebtWhat":     "Fresh borrowing, new card charges and interest added to what you owe.",
	"nws.legRevaluationWhat": "Balance updates on property, vehicles and cash — a re-appraisal, depreciation, a reconcile.",
	"nws.legResidualWhat":    "The part the steps above don't account for: transactions excluded from reports, currency conversion, and unusual debt movements. Shown rather than hidden.",

	// ── TWO SIDES
	"nws.sidesTitle":  "Two sides",
	"nws.sidesNote":   "What you own stacked upward, what you owe stacked downward, and your net worth running between them. The gap is the story.",
	"nws.sidesAria":   "Mirrored area chart: assets %s above the line, liabilities %s below, net worth %s through the middle.",
	"nws.sidesEmpty":  "Not enough history yet to draw the two sides — check back after another month.",
	"nws.legendEntry": "%s %s (%d%%)",
	"nws.legendNet":   "Net worth",

	// ── The read: takeaway + ratios
	"nws.readTitle":     "What it means",
	"nws.takeUp":        "Up %s over %s — mostly from %s, which accounts for %d%% of the move.",
	"nws.takeDown":      "Down %s over %s — mostly from %s, which accounts for %d%% of the move.",
	"nws.takeUpPlain":   "Up %s over %s.",
	"nws.takeDownPlain": "Down %s over %s.",
	"nws.takeFlat":      "Holding steady at %s across %s.",

	"nws.causeMoneyKept":   "the money you kept",
	"nws.causeMarket":      "the market moving your investments",
	"nws.causeDebtPaid":    "paying down debt",
	"nws.causeNewDebt":     "taking on new debt",
	"nws.causeRevaluation": "what your property and vehicles are now worth",

	"nws.ratioLiquid":      "Liquid share",
	"nws.ratioLiquidDef":   "Spendable cash as a share of everything you own.",
	"nws.ratioRunway":      "Cash runway",
	"nws.ratioRunwayDef":   "How many months of your typical spending your cash would cover.",
	"nws.ratioDebt":        "Debt to assets",
	"nws.ratioDebtDef":     "What you owe as a share of what you own.",
	"nws.ratioUnknown":     "—",
	"nws.ratioUnknownRead": "Not enough to work this out yet.",
	"nws.runwayValue":      "%s months",

	"nws.readLiquidStrong": "Plenty on hand — %s of your wealth is cash you can actually reach.",
	"nws.readLiquidOk":     "A normal balance — %s of what you own is spendable cash.",
	"nws.readLiquidWatch":  "Most of your wealth isn't spendable; only %s is cash. Fine if it's a home, tight if you need it fast.",
	"nws.readLiquidAlarm":  "Very little of what you own is reachable — just %s in cash.",

	"nws.readRunwayStrong":  "Comfortable: that's several months of your usual %s a month.",
	"nws.readRunwayOk":      "A reasonable cushion against your usual %s a month.",
	"nws.readRunwayWatch":   "Thin: less than three months of your usual %s a month.",
	"nws.readRunwayAlarm":   "Under a month of your usual %s spending — this is the number worth fixing first.",
	"nws.readRunwayUnknown": "No spending history yet to measure this against.",

	"nws.readDebtStrong": "You owe little against what you own.",
	"nws.readDebtOk":     "A normal amount of borrowing for a household that owns property.",
	"nws.readDebtWatch":  "Borrowing is a large share of what you own — worth watching, not panicking about.",
	"nws.readDebtAlarm":  "You owe more than most of what you own.",

	// ── Detail sections
	"nws.indexAria":      "Jump to a section",
	"nws.secStand":       "Where you stand",
	"nws.secStandNote":   "The balance sheet, as of today.",
	"nws.secChanged":     "What changed",
	"nws.secChangedNote": "Every step of the move across %s, and every account behind it.",
	"nws.secOwn":         "What you own",
	"nws.secOwnNote":     "%s in assets, measured against each other rather than against your debts.",
	"nws.secOwe":         "What you owe",
	"nws.secOweNote":     "%s owed, measured against each other rather than against what you own.",
	"nws.secHistory":     "History",
	"nws.secHistoryNote": "The same two sides, month by month, with the figures behind the chart.",
	"nws.secHealth":      "Ratios & health",
	"nws.secHealthNote":  "What each ratio measures, what yours is, and what it means.",

	"nws.colLeg":     "Step",
	"nws.colEffect":  "Effect",
	"nws.colAccount": "Account",
	"nws.colKind":    "Kind",
	"nws.colMoved":   "Moved",
	"nws.colGroup":   "Group",
	"nws.colShare":   "Share",
	"nws.colAmount":  "Amount",
	"nws.colWhen":    "When",
	"nws.colRatio":   "Ratio",
	"nws.colMeans":   "What it measures",
	"nws.colNow":     "Yours",

	"nws.moversTitle":  "Biggest movers",
	"nws.moversNote":   "All %d accounts that moved, largest first.",
	"nws.moversEmpty":  "No account changed over this period.",
	"nws.sideAccounts": "All %d accounts, largest first.",
}

func init() {
	for k, v := range netWorthRedesignKeys {
		english[k] = v
	}
}
