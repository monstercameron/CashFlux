// SPDX-License-Identifier: MIT

package i18n

// reportsVitalsKeys holds the copy for the Annual Review's "00 · Where you
// stand" section — the position snapshot (cash-flow capacity, the liquid
// cushion, debt & credit) that opens the document before the year's story.
// Merged via init so this file does not touch en.go.
var reportsVitalsKeys = Catalog{
	"rpta.idxStand":    "Where you stand",
	"rpta.secStand":    "Where you stand",
	"rpta.secStandSub": "Today's position, before the year's story — what a month looks like, how deep the cushion is, and what's owed.",
	"rpta.standBasis":  "Monthly figures are averages over your last %d active months.",

	// Column heads.
	"rpta.standColFlow":    "Cash flow",
	"rpta.standColCushion": "The cushion",
	"rpta.standColDebt":    "Debt & credit",

	// Cash flow.
	"rpta.standIncomeK":  "Money in / month",
	"rpta.standSpendK":   "Money out / month",
	"rpta.standKeptK":    "Kept / month",
	"rpta.standKeptR":    "About %s a year at this pace.",
	"rpta.standRateK":    "Savings rate",
	"rpta.standRateR":    "Target: keep %d%% or more of what you earn.",
	"rpta.standFreeK":    "Free after debt minimums",
	"rpta.standFreeR":    "What's left once spending and required debt payments are both covered.",
	"rpta.standNoIncome": "No income in the last year of records, so the capacity figures would be guesses. Add income transactions to see them.",

	// The cushion.
	"rpta.standEssK":      "One essential month",
	"rpta.standEssR":      "Fixed commitments (%s) plus essential spending (%s) — what a bare month costs.",
	"rpta.standEssRFixed": "What a bare month costs — from your recurring commitments; no essential-classified spending recorded yet.",
	"rpta.standEssRSpend": "What a bare month costs — from essential-classified spending; no recurring commitments recorded yet.",
	"rpta.standLiquidK":   "Liquid cash",
	"rpta.standCoverK":    "Coverage",
	"rpta.standCoverR":    "Six essential months is the full cushion; three is the floor.",
	"rpta.standFundK":     "%d-month fund",
	"rpta.standFundShort": "%s short of the target.",
	"rpta.standFundMet":   "Met — %s past the target.",
	"rpta.standFundExact": "Met exactly.",
	"rpta.standRunwayK":   "Runway after debt service",
	"rpta.standRunwayR":   "If income stopped: liquid cash against average spending plus debt minimums.",
	"rpta.standNoCushion": "Not enough recurring bills or classified spending yet to size an essential month.",

	// Debt & credit.
	"rpta.standDebtK":       "Total debt",
	"rpta.standDebtMortR":   "%s of it sits outside the mortgage.",
	"rpta.standMinsK":       "Required payments / month",
	"rpta.standMinsR":       "%s a year committed before any extra paydown.",
	"rpta.standDtiK":        "Share of income",
	"rpta.standDtiR":        "Required minimums against monthly income — under %d%% is the common comfort line.",
	"rpta.standAprK":        "Blended interest rate",
	"rpta.standAprR":        "Costing about %s a month in interest to stand still.",
	"rpta.standPayoffK":     "Debt-free horizon",
	"rpta.standPayoffYrMo":  "%d yr %d mo",
	"rpta.standPayoffMo":    "%d mo",
	"rpta.standPayoffNever": "Never at minimums",
	"rpta.standPayoffR":     "Paying minimums only, highest rate first.",
	"rpta.standPayoffXMort": " The mortgage isn't included.",
	"rpta.standPayoffBadR":  "The minimums don't outpace the interest — only extra payments clear this.",
	"rpta.standUtilK":       "Card utilization",
	"rpta.standUtilR":       "Using %s of the %s limit — %s still open.",
	"rpta.standUtilNoLim":   "No card limits recorded, so utilization can't be measured. Add limits on the accounts page.",
	"rpta.standNoDebt":      "Nothing owed — no debts are tracked.",

	// Shared.
	"rpta.standMonthsVal":  "%s mo",
	"rpta.standMeterTitle": "%s now · target %s",
}

func init() {
	for k, v := range reportsVitalsKeys {
		english[k] = v
	}
}
