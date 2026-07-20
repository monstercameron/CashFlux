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
	"nws.spanMonth":       "this month",
	"nws.spanMonths":      "over the last %d months",
	"nws.spanYears":       "over the last two years",
	"nws.spanAll":         "since your records began",
	"nws.deltaTitle":      "How your net worth has moved %s",

	// ── THE BRIDGE
	"nws.bridgeTitle": "What moved it",
	"nws.bridgeNote":  "Every step from where you stood %s to where you stand now — including the part we can't attribute.",
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
	"nws.sidesNote":   "What you own on top, what you owe beneath — the space between them is your net worth.",
	"nws.sidesAria":   "Chart of two boundaries: assets %s on top, liabilities %s beneath, with the gap between them — your net worth — going from %s to %s.",
	"nws.sidesFloor":  "The scale starts at %s, not zero, so the movement in both sides stays visible.",
	"nws.sidesEmpty":  "Not enough history yet to draw the two sides — check back after another month.",
	"nws.annoNet":     "Net worth",
	"nws.gapWas":      "Gap was %s",
	"nws.gapNow":      "Gap is %s",
	"nws.stripOwn":    "What you own",
	"nws.stripOwe":    "What you owe",
	"nws.legendEntry": "%s %s (%d%%)",

	// ── The "?" explainers. Plain language: what the picture shows and how to
	// read it, never how it is computed.
	"nws.explainAria":    "How to read the %s chart",
	"nws.explainBridge1": "This walks from what you were worth at the start of the period to what you're worth now, one cause at a time.",
	"nws.explainBridge2": "Green steps pushed your net worth up, grey steps pulled it down. The taller the step, the bigger its part in the change.",
	"nws.explainBridge3": "The steps add up exactly to the final figure — including \"Unexplained\", which is whatever we couldn't confidently put down to one cause. We show it rather than quietly folding it into a neighbour.",
	"nws.explainSides1":  "The top line is everything you own. The bottom line is everything you owe. The shaded space between them is your net worth.",
	"nws.explainSides2":  "So when the gap grows, you're getting wealthier — whether that's because the top line rose, the bottom line fell, or both.",
	"nws.explainSides3":  "The scale starts near your lowest figure rather than at zero, so small month-to-month movements are still visible. The dollar labels down the left tell you where you actually are.",
	"nws.explainSides4":  "Beneath the chart, each dot marks a round figure your net worth passed, and the number between two dots is how long that climb took — \"14 mo\" means fourteen months from one to the next. Marks on the chart itself show where each of those crossings happened; a dashed one is a setback rather than a gain.",

	// ── What the figure rests on. A disclosure, not a nag: plain about what is
	// current and what is not, and never claiming a certainty the data cannot
	// support (the app stores exchange rates but not when they were captured,
	// so the FX line says where they come from, not how fresh they are).
	"nws.dqTitle":          "What this figure rests on",
	"nws.dqAria":           "What this figure rests on",
	"nws.dqClean":          "Based on %d accounts",
	"nws.dqStaleSummary":   "Based on %d accounts · %d need updating",
	"nws.dqIncluded":       "%d accounts are counted in this figure.",
	"nws.dqManual":         "Your oldest hand-entered value is %s, last confirmed %s.",
	"nws.dqManualDominant": "Your oldest hand-entered value is %s, last confirmed %s — and it's about %d%% of everything you own, so this figure leans heavily on it being right.",
	"nws.dqDominant":       "%s is about %d%% of everything you own, so this figure moves largely with it.",
	"nws.dqNever":          "never",
	"nws.dqFx":             "Amounts in %s are converted to %s using the exchange rates saved in your settings — we don't track when those were last updated, so check them if they may have moved.",
	"nws.dqExcludedChoice": "%s you've chosen to leave out of net worth.",
	"nws.dqExcludedNoRate": "%s couldn't be included: there's no exchange rate for its currency.",
	"nws.dqAllCurrent":     "Every account is up to date.",
	"nws.dqUpdate":         "Open accounts to edit a figure",
	"nws.dqColAccount":     "Account",
	"nws.dqColConfirmed":   "Last confirmed",
	"nws.dqColAge":         "Age",
	"nws.dqConfirmAll":     "Confirm all %d as current",
	"nws.and":              "and",

	// ── Investigating a number in place
	"nws.drillAria":         "Show the detail behind %s",
	"nws.factKind":          "Kind",
	"nws.factOpening":       "Balance at the start",
	"nws.factClosing":       "Balance now",
	"nws.factFlow":          "From money moving in and out",
	"nws.factAdjusted":      "From balance updates you entered",
	"nws.factCurrency":      "Held in",
	"nws.factSource":        "Balance comes from",
	"nws.factSourceTracked": "Its transactions",
	"nws.factSourceManual":  "A figure you entered",
	"nws.factConfirmed":     "Last confirmed",
	"nws.factRate":          "Interest rate",
	"nws.factLedger":        "See these transactions",
	"nws.factLedgerTitle":   "Open the ledger filtered to %s over this period",
	"nws.factOpenAccount":   "Open account",
	"nws.legContributors":   "Which accounts made up %s:",
	"nws.legNoContributors": "No single account accounts for this step.",

	// ── The read: takeaway + ratios
	"nws.readTitle":     "What it means",
	"nws.takeUp":        "Up %s %s. The biggest single cause was %s, which was %d%% of everything that moved.",
	"nws.takeDown":      "Down %s %s. The biggest single cause was %s, which was %d%% of everything that moved.",
	"nws.takeUpPlain":   "Up %s %s.",
	"nws.takeDownPlain": "Down %s %s.",
	"nws.takeFlat":      "Holding steady at %s %s.",

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

	"nws.readLiquidStrong": "%s of what you own is cash you can reach quickly.",
	"nws.readLiquidOk":     "%s of what you own is cash you can reach quickly; the rest is held in other things.",
	"nws.readLiquidWatch":  "Only %s of what you own is cash. The rest you'd have to sell to spend.",
	"nws.readLiquidAlarm":  "Very little of what you own is cash you could reach quickly — just %s.",

	"nws.readRunwayStrong":  "Measured against your average spending of %s a month over the last three months.",
	"nws.readRunwayOk":      "Measured against your average spending of %s a month over the last three months.",
	"nws.readRunwayWatch":   "Under three months at your average spending of %s a month over the last three months.",
	"nws.readRunwayAlarm":   "Under one month at your average spending of %s a month over the last three months.",
	"nws.readRunwayUnknown": "No spending history yet to measure this against.",

	"nws.readDebtStrong": "You owe little against what you own.",
	"nws.readDebtOk":     "Your borrowing is a moderate share of what you own.",
	"nws.readDebtWatch":  "Your borrowing is a large share of what you own.",
	"nws.readDebtAlarm":  "You owe more than most of what you own.",

	// ── Milestones
	"nws.milestonesTitle":   "Milestones",
	"nws.milestoneUp":       "Passed %s in %s.",
	"nws.milestoneDown":     "Fell back below %s in %s.",
	"nws.milestonePos":      "Net worth turned positive in %s.",
	"nws.milestoneNeg":      "Net worth turned negative in %s.",
	"nws.milestoneHigh":     "Reached its highest so far, %s, in %s.",
	"nws.milestoneReversal": "Fell from %s to %s by %s.",
	"nws.milestonesNone":    "No round figures were crossed over this period.",
	// ── The pace rail. Every line states what happened or what a projection
	// assumes; none of it advises, congratulates, or promises a date.
	"nws.paceMonths":   "%d mo",
	"nws.paceNext":     "Next %s: about %d months at your recent pace",
	"nws.paceStalled":  "Not gaining at your recent pace",
	"nws.paceSummary":  "%s to %s took %d months.",
	"nws.paceFallOnce": "It fell back once along the way.",
	"nws.paceFalls":    "It fell back %d times along the way.",
	"nws.paceNone":     "No round figures reached over this period.",

	// The projection's method, stated rather than trusted.
	"nws.paceExplainTitle": "How this projection works",
	"nws.paceLookback":     "the last %d months",
	"nws.explainPace1":     "It takes your net worth over %s and averages the change: %s a month.",
	"nws.explainPace2":     "That average counts everything that moved your net worth in the period — money you kept, debt you paid down, and any change in what your property and investments are valued at, converted to your base currency.",
	"nws.explainPace3":     "It then draws a straight line from where you stand to the next round figure. It is one scenario, not a forecast: a month of unusual spending or a re-valued asset moves it. If your recent pace is flat or downward, no date is offered at all.",

	"nws.historyShow":    "Show the numbers · %d months",
	"nws.historyHide":    "Hide the numbers",
	"nws.historyCaption": "Net worth month by month: what you own, what you owe, and the difference.",

	"nws.milestonesShowAll": "Show all %d milestones",
	"nws.milestonesHide":    "Hide the full list",
	"nws.winAll":            "All time",
	"nws.agoAll":            "when your records began",

	// ── Detail sections
	"nws.indexAria":       "Jump to a section",
	"nws.backTop":         "Top",
	"nws.backGlance":      "Back to Glance",
	"nws.backGlanceTitle": "Return to the one-screen summary",
	"nws.secStand":        "Where you stand",
	"nws.secStandNote":    "The balance sheet, as of today.",
	"nws.secChanged":      "What changed",
	"nws.secChangedNote":  "Every step of the move %s, and every account behind it.",
	"nws.secOwn":          "What you own",
	"nws.secOwnNote":      "%s in assets, measured against each other rather than against your debts.",
	"nws.secOwe":          "What you owe",
	"nws.secOweNote":      "%s owed, measured against each other rather than against what you own.",
	"nws.secHistory":      "History",
	"nws.secHistoryNote":  "The same two sides, month by month, with the figures behind the chart.",
	"nws.secHealth":       "Ratios & health",
	"nws.secHealthNote":   "What each ratio measures, what yours is, and what it means.",

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
