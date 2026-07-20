// SPDX-License-Identifier: MIT

package i18n

// rhythmKeys holds the English strings for the unified Bills & Recurring
// surface (the "month's rhythm" page: /recurring, /bills, /subscriptions). New
// copy lives here under the rhythm.* prefix; unchanged copy is reused from the
// recurring.* / bills.* / subs.* / pricecreep.* catalogs directly at the call
// site. Merged via init so this file does not touch en.go (mirrors the
// en_recurringsurface.go pattern).
var rhythmKeys = Catalog{
	// ── Hero: the tideline ──────────────────────────────────────────────────
	"rhythm.heroTitle":     "This month's rhythm",
	"rhythm.heroNote":      "Bills, subscriptions, and paychecks that repeat — and how your cash rides the pay cycle.",
	"rhythm.tideAria":      "Projected cushion over your pay cycle",
	"rhythm.tideNoIncome":  "No income scheduled yet, so there's no cushion line to draw. Add a paycheck and the pay cycle appears here.",
	"rhythm.tideAddIncome": "Add income",
	"rhythm.tideTiny":      "Add a few recurring items to see the month's shape.",
	"rhythm.pinch":         "Tightest: %s · %s cushion",
	"rhythm.pinchNeg":      "Goes negative %s · %s short",
	// The calm state still reports the low point — a pinch flag that fires every
	// cycle is decoration, so it only appears when the cycle is genuinely tight.
	"rhythm.pinchCalm": "No tight spots this cycle · lowest %s on %s",
	"rhythm.statNet":   "Net / mo",
	"rhythm.statIn":    "In / mo",
	"rhythm.statOut":   "Out / mo",

	// ── Overdue strip ───────────────────────────────────────────────────────
	"rhythm.overdueSummary": "%s overdue · %s total",
	"rhythm.postNow":        "Post now",

	// ── Review strip ────────────────────────────────────────────────────────
	"rhythm.reviewTitle": "Waiting for your review",
	// The header counts what is actually reviewable HERE — the same number the
	// lane header and the pager count. Demoted signals get their own figure and
	// their own route (rhythm.weakSignalsLink) rather than being folded into a
	// headline total that leads to a five-item queue.
	"rhythm.reviewTitleCount": "Waiting for your review · %d to review",
	"rhythm.weakSignalsLink":  "%d weaker signals",
	"rhythm.weakSignalsTitle": "See the patterns we judged too weak to propose, in Detection preferences",
	"rhythm.reviewNote":       "Repeating charges we found in your history. Confirm the real ones so they join your plan.",
	"rhythm.smartMark":        "Smart",
	"rhythm.smartPlusMark":    "Smart+",
	"rhythm.reviewSmartGroup": "found %s in your history",
	// The Smart+ lane's number counts the leftover patterns it SENDS, and the list
	// is the same before and after the round trip — so it never counted anything
	// "found by AI". Each state says what its number actually is.
	"rhythm.reviewPlusSent":  "%s sent for a deeper look",
	"rhythm.reviewPlusRead":  "%s looked at, still checked here locally",
	"rhythm.confirm":         "Confirm",
	"rhythm.notRecurring":    "Not recurring",
	"rhythm.seeEvidence":     "See transactions",
	"rhythm.hideEvidence":    "Hide transactions",
	"rhythm.lookDeeper":      "Look deeper with Smart+ — about %d tokens, on your OpenAI key",
	"rhythm.lookDeeperNoKey": "Add an OpenAI key in Settings to look deeper with Smart+",
	// Disabled state: the button says what it would do, the sentence says why it
	// can't and where to fix it — never an enabled-looking button beside a
	// "you need a key" note.
	"rhythm.lookDeeperLabel":    "Look deeper with Smart+",
	"rhythm.lookDeeperNeedsKey": "Needs your own OpenAI key — about %d tokens for this scan.",
	"rhythm.lookDeeperSettings": "Add one in Settings",
	"rhythm.lookDeeperBusy":     "Looking deeper…",
	"rhythm.verifiedLocally":    "verified locally",
	"rhythm.noLocalConfirm":     "no local way to confirm",
	"rhythm.reviewNone":         "Nothing new to review — every repeating charge we found is already on your plan.",

	// The way back out of "Not recurring" — a quiet section in Detection
	// preferences, beside the other judgments the user can inspect and change.
	"rhythm.hiddenLabel":     "Hidden as not recurring (%d)",
	"rhythm.hiddenDesc":      "Charges you told us don't repeat, so we stopped proposing them. Changed your mind about one? Show it again and it goes back in the review queue.",
	"rhythm.unsuppress":      "Show this again",
	"rhythm.unsuppressTitle": "Let this charge be proposed for review again",

	// Evidence sentence fragments (composed in order in Go).
	"rhythm.evPayments":    "%d payments",
	"rhythm.evAround":      "%s around the %s",
	"rhythm.evOn":          "%s on %s",
	"rhythm.evPostsBy":     "usually posts by the %s",
	"rhythm.evEvery":       "%s every time",
	"rhythm.evAbout":       "about %s",
	"rhythm.evLast":        "last %s",
	"rhythm.evScheduledAs": "detected %s, tracked as %s",
	"rhythm.evTxnMissing":  "This charge is no longer in your transactions",

	// Detected-rhythm labels (lowercase, for the evidence sentence — distinct
	// from the recurring.cadence* adjectives used elsewhere).
	"rhythm.rcWeekly":      "weekly",
	"rhythm.rcBiweekly":    "every two weeks",
	"rhythm.rcSemimonthly": "twice a month",
	"rhythm.rcMonthly":     "monthly",
	"rhythm.rcEvery4Weeks": "every 4 weeks",
	"rhythm.rcQuarterly":   "quarterly",
	"rhythm.rcSemiannual":  "twice a year",
	"rhythm.rcAnnual":      "yearly",
	"rhythm.rcUnknown":     "irregularly",

	// ── Up next — the agenda ────────────────────────────────────────────────
	"rhythm.agendaTitle": "Up next",
	// The note names the window each view actually draws. The compact list runs
	// well past one pay cycle, so a monthly bill appears in it more than once — the
	// month headings are what stop that reading as owing it twice.
	"rhythm.agendaNote":    "Everything due in the next %d days, income included. Grouped by month, so a monthly bill appears once in each.",
	"rhythm.agendaNoteCal": "Everything due, a month at a time, income included.",
	"rhythm.agendaNone":    "Nothing scheduled ahead.",
	"rhythm.viewAria":      "Agenda view",
	"rhythm.viewCompact":   "Compact",
	"rhythm.viewCalendar":  "Calendar",
	"rhythm.showAll":       "Show all %d",
	"rhythm.showFewer":     "Show fewer",
	"rhythm.calMore":       "+%d more",
	"rhythm.calMissed":     "This one went by without being paid",
	"rhythm.calPast":       "Already gone by",

	// Posting-mode badges.
	"rhythm.modeAuto":       "Auto",
	"rhythm.modeWatch":      "Watching",
	"rhythm.modeManual":     "Manual",
	"rhythm.modeAutoHint":   "Posts to the ledger automatically when it's due.",
	"rhythm.modeWatchHint":  "The biller charges this; we match it from your transactions.",
	"rhythm.modeManualHint": "You mark this one paid.",

	// ── The lineup — roster ─────────────────────────────────────────────────
	"rhythm.rosterTitle":  "The lineup",
	"rhythm.rosterNote":   "Everything that repeats, heaviest first.",
	"rhythm.rosterNone":   "Nothing here yet.",
	"rhythm.lensAll":      "All",
	"rhythm.lensBills":    "Bills",
	"rhythm.lensSubs":     "Subscriptions",
	"rhythm.lensIncome":   "Income",
	"rhythm.subsSubtotal": "%s / mo · %s / yr",
	"rhythm.sortAria":     "Sort the lineup",
	"rhythm.sortSize":     "By size",
	"rhythm.sortNext":     "By next date",
	"rhythm.sortName":     "By name",
	"rhythm.sortTrend":    "By trend",
	"rhythm.shareOfIn":    "%.0f%% of income",
	"rhythm.perMonth":     "%s / mo",
	"rhythm.perMonthVar":  "about %s / mo",
	"rhythm.anchorTitle":  "Linked to %s — open the account",
	"rhythm.creepTitle":   "This charge has been creeping up",
	"rhythm.nextOn":       "next %s",
	"rhythm.pausedTag":    "Paused",
	"rhythm.pause":        "Pause",
	"rhythm.resume":       "Resume",
	"rhythm.cancelWatch":  "Cancel — keep watching",
	"rhythm.copyVar":      "Copy formula variable",
	"rhythm.copiedVar":    "Copied %s",
	"rhythm.watchTail":    "Watching after cancellation (%d)",
	"rhythm.watchStatus":  "Cancelled — still watching for charges",

	// ── Findings ────────────────────────────────────────────────────────────
	"rhythm.findingsTitle": "Worth a look",
	"rhythm.findCharged":   "%s charged %s after you cancelled it.",
	"rhythm.findDispute":   "Add a to-do",
	"rhythm.findStopped":   "%s seems to have stopped — no charge for %d cycles.",
	"rhythm.findPause":     "Pause it",
	"rhythm.disputeTask":   "Dispute %s — charged %s after cancellation",

	// ── Utilities toolbar ───────────────────────────────────────────────────
	"rhythm.toolsMetrics":     "Schedule metrics",
	"rhythm.toolsMetricsHide": "Hide metrics",
	"rhythm.toolsDetection":   "Detection preferences",
	"rhythm.toolsCsv":         "Download CSV",

	// Deep-link scroll anchors carry these as accessible section landmarks.
	"rhythm.jumpAgenda": "Jump to what's up next",
	"rhythm.jumpRoster": "Jump to the lineup",
}

func init() {
	for k, v := range rhythmKeys {
		english[k] = v
	}
}
