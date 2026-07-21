// SPDX-License-Identifier: MIT

package i18n

// healthAnalysisKeys holds English copy for the /health analysis surfaces added
// in the redesign: the resilience runway, the interactive stress tests, the money-
// leaks read (recurring load + spending creep), and the trend-narrated history.
// Kept in its own file (not en.go) so it doesn't touch the concurrent-WIP catalog.
var healthAnalysisKeys = Catalog{
	// Resilience runway. %s = a duration like "4 mo" / "1 yr 2 mo".
	"healthx.runwayHero": "Resilient for %s with no income",
	"healthx.runwayLead": "With no income at all, your %s cash buffer would cover everything for %s.",

	// Stress-test tile.
	"healthx.stressTitle":   "What if something goes wrong?",
	"healthx.stressHint":    "Try a shock and read the concrete outcome. Nothing here changes your data.",
	"healthx.dropLabel":     "Pay cut",
	"healthx.surpriseLabel": "Surprise bill",
	"healthx.rateLabel":     "Card rate hike",
	// %d = drop percent, %s = duration.
	"healthx.dropNegative": "A %d%% pay cut and you'd burn through the buffer in about %s.",
	// %d = drop percent, %s = remaining monthly surplus.
	"healthx.dropOk": "A %d%% pay cut still leaves %s a month to spare.",
	// %s = surprise amount, %s = amount pushed to cards, %s = extra monthly interest.
	"healthx.surpriseDebt": "A %s surprise would push %s onto the cards — about %s a month in new interest.",
	// %s = surprise amount, %s = buffer remaining.
	"healthx.surpriseOk": "A %s surprise leaves %s in the buffer, nothing on the cards.",
	// %d = points, %s = extra monthly interest, %s = extra annual interest.
	"healthx.rateOut":     "Rates +%d points would add %s a month (%s a year) in interest.",
	"healthx.rateNoCards": "No card balances to bite — a rate hike wouldn't change anything.",

	// Money-leaks tile.
	"healthx.leaksTitle":        "Where the money leaks",
	"healthx.recurringSubtitle": "Recurring commitments",
	"healthx.recurringPerMo":    "%s / mo",                                 // %s = total monthly recurring
	"healthx.recurringCount":    "across %d recurring charges · %s a year", // %d count, %s annual
	"healthx.recurringShare":    "That's %s%% of your income.",             // %s = share percent
	"healthx.recurringBiggest":  "Biggest:",
	"healthx.noRecurring":       "No recurring charges on file yet.",
	"healthx.creepSubtitle":     "Spending creep",
	"healthx.creepHint":         "Categories running above their own usual — often the easiest savings to find.",
	"healthx.creepDetail":       "%s lately vs %s usual", // %s recent avg, %s median
	"healthx.creepSave":         "save %s/mo",            // %s monthly saving
	"healthx.noCreep":           "Nothing creeping — recent spending is in line with your norm.",
	"healthx.manageRecurring":   "Bills & recurring",

	// Trend-narrated history takeaways. %d = streak length (months), %d = latest score.
	"healthx.historyStreakUp":   "Up %d months running — now %d.",
	"healthx.historyStreakDown": "Slipping %d months running — now %d.",
	"healthx.historyRecover":    "Recovering — up %d months after a dip, now %d.",

	// Hero "why this score" contribution breakdown.
	"healthx.whyScore":     "Why this score",
	"healthx.whyScoreAria": "Score of %d, broken down by each factor's point contribution", // %d = score

	// Accessible names for the stress chips (the bare "10%" / "$500" / "+5" don't say
	// which shock). %d / %s = the shock size.
	"healthx.dropAria":     "Model a %d%% pay cut",
	"healthx.surpriseAria": "Model a %s surprise bill",
	"healthx.rateAria":     "Model a %d-point card rate increase",

	// Savings factor: name the averaging window so it isn't read as the current-period
	// savings_rate formula of the same name.
	"healthx.savingsPeriod": "Averaged over the last 3 full months",

	// Accessible name for each factor's scoring disclosure. %s = factor label.
	"healthx.curveAria": "How %s is measured and scored",
	// Accessible name for a clickable spending-creep row. %s = category name.
	"healthx.creepAria": "Review %s transactions",
}

func init() {
	for k, v := range healthAnalysisKeys {
		english[k] = v
	}
}
