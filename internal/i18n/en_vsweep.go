// SPDX-License-Identifier: MIT

package i18n

import "maps"

// vSweepKeys holds the English copy introduced by the 2026-07-03 world-class
// visual/UX sweep's remaining tickets (C340–C362). Kept in its own file and
// merged via init so it never collides with in-flight edits to en.go, mirroring
// en_uxsweep.go.
var vSweepKeys = Catalog{
	// ── C341: the one shared net-worth-change sentence ──────────────────────
	// Every surface that prints a net-worth delta renders it through
	// screens.nwChangeSub, so the window is always named and the wording never
	// varies. %d = whole percent (always positive; the arrow carries direction),
	// first %s = the amount as a magnitude, last %s = the window name.
	"nw.windowMonthToDate": "this month",
	"nw.changeUp":          "▲ %d%% (+%s) %s",
	"nw.changeDown":        "▼ %d%% (−%s) %s",
	"nw.changeUpAmount":    "▲ +%s %s",
	"nw.changeDownAmount":  "▼ −%s %s",
	"nw.changeNone":        "No change %s",
	"nw.changeUnknown":     "Change %s isn't available yet",
	// Stat label on the Reports net-worth tab. It used to read "Change this
	// period" while showing last month's step; it now names its real window.
	"nw.changeMonthLabel": "Change this month",

	// ── C342/C343: name the window over every figure that has one ───────────
	// Caption above the dashboard hero's income / spending / net / savings-rate
	// row. %s is the selected period ("Jul 2026").
	"home.statsWindow": "For %s",
	// /health factor measurement windows. The model names the span as a key;
	// these are the only place it becomes English.
	"healthx.windowTrailing3mo":   "Averaged over the last 3 full months",
	"healthx.windowCurrentPeriod": "This period so far",
	"healthx.windowAsOfToday":     "As of today",
	"healthx.windowPriorPeriod":   "Last completed period — this one has barely started",
	// Tooltip on the period chip a windowed dashboard tile always wears — the
	// mirror of the "Today" chip a current-state tile wears when paged away.
	"widget.windowBadgeTitle": "This tile's figures cover the selected period",

	// ── C340: one row for one real payment ──────────────────────────────────
	// A liability's own statement bill and the recurring flow the household
	// created to pay it are the same money. The surviving row names what it
	// absorbed so the merge reads as a merge, not as a missing bill. %s is the
	// liability account's name.
	"bills.covers":     "covers %s",
	"bills.coversHint": "This payment settles %s — its statement bill is shown here, not separately",

	// ── C344: a period too young to judge says so ───────────────────────────
	// Budget card status while barely any of the period has run. "On track" on
	// day 3 is a claim about the calendar, not about the household.
	"budgets.periodJustStarted": "Period just started",
	// The reading that IS worth something while the period is young: what last
	// period actually came to. %s = last period's spend, %s = the "$X under" /
	// "$X over" gap against this period's budget.
	"budgets.priorPeriodOutcome": "Last period: %s (%s)",

	// ── C345: a due-dated alert is stamped with its deadline ────────────────
	// The feed is rebuilt on boot, so every row read "just now" — a column of
	// identical timestamps on the one surface whose job is ranking by urgency.
	// %d = whole days past the due date.
	"notifications.overdueBy": "overdue by %d days",

	// ── C347: name why a detection isn't a subscription ─────────────────────
	// "Review" reads as "we aren't sure yet". The real answer for a recurring
	// grocery run is that it isn't billed at a set price, which is what the word
	// subscription means. %d = the largest deviation from the median charge.
	"subs.varies":     "varies",
	"subs.variesHint": "Charged a different amount each time (up to %d%% apart) — this is recurring spending, not a set-price subscription",

	// ── C353: /allocate meters are scores, not rates ────────────────────────
	// The criterion axes are normalized 0–100 scores. Printing them with a
	// percent sign made "Pay down Mortgage — RETURNS 27%" read as a claim about
	// a 4.1% mortgage, and "RETURNS 100%" read as an impossible one. %d = the
	// score.
	"allocate.scoreOutOf": "%d/100",
	"allocate.scoreHint":  "A ranking score out of 100 across your chosen criteria — not a rate of return",
	// The real finance figure beside the Returns score. %.1f = annual percent.
	"allocate.realAPRAsset": "%.1f%% expected",
	"allocate.realAPRDebt":  "%.1f%% APR saved",
	// Screen-reader label for the ranking meter. %d = the score out of 100.
	"allocate.scoreMeterAria": "Ranking score %d out of 100",

	// ── C354: two 0–100 scores must not share a name ────────────────────────
	// /health showed 73 "Good" with a green ring and /credit showed 55 "Good"
	// with a green ring, both labelled "…health". A reader comparing them had no
	// way to tell they were different scales measuring different things. The
	// credit proxy is about HABITS — utilization, payment history, age — so it
	// says so, and "Financial health" keeps the name it earned.
	"nav.credit":               "Credit habits",
	"credit.pageTitle":         "Credit habits",
	"credit.ringLabel":         "Credit-habits score: %d out of 100 — %s",
	"detail6.creditScoreLabel": "Credit-habits score (CashFlux estimate — not a bureau score)",

	// ── C348: name the overlap between the page's three lists ───────────────
	// The same subscriptions appear here, in "Renewing soon" and in "Recent
	// price changes". That repetition is deliberate; what made it read as
	// duplication is that the sections never mentioned each other. %d = a count.
	"subs.xlinkSoon":    "%d of these renew within a week →",
	"subs.xlinkChanges": "%d changed price recently →",
	"subs.xlinkBack":    "← Back to all subscriptions",

	// ── C358: a plan card states its arc, not just its ending ───────────────
	// "($25,100.00)" alone reads as an alarm. A savings plan that spends its
	// savings on the thing it was saving for ends below where it began BY
	// DESIGN. %s = start, %s = end, %s = when the horizon ends.
	"plans.arc":          "Starts %s → ends %s by %s",
	"plans.arcNoHorizon": "the end of the plan",

	// ── C359: each page states its own question ─────────────────────────────
	// /debt showed /credit's whole panel verbatim. It carries the headline as
	// context for the payoff plan now, and hands off for the rest.
	"credit.seeFullPanel":   "See what's driving this score, card by card →",
	"debt.creditSummarySub": "How your card habits are scoring — the detail lives on Credit habits",
	// A page's subtitle is where it declares its own question (R58). These three
	// described their CONTENTS, which is how three surfaces came to look like
	// they were doing each other's jobs.
	"screen.netWorthSub": "What moved your net worth, and what it means",
	"screen.insightsSub": "The assistant's read on your spending — a tab of the assistant hub",
	"screen.creditSub":   "What your card habits are doing to your score — local, private, no bureau",

	// ── C360: sweep polish ──────────────────────────────────────────────────
	// /split's running-balance rows printed the amount in the label AND in the
	// amount column. The label says who and which way. %s = a member's name.
	"split.netOwes":   "%s owes",
	"split.netIsOwed": "%s is owed",
	// /plans built this sub-price by concatenating English around a catalog
	// value, which the hardcoded-copy scanner does not see in an argument
	// position. %s = the monthly price.
	"plans.orMonthly": "or %s billed monthly",
	// The spending breakdown with exactly one category: a full-width bar reading
	// "Groceries 100%" claims a composition the data does not have. %s = the
	// category name.
	"dashboard.breakdownSingle": "Everything spent this period is in %s so far.",

	// ── C361: the boot chrome the Go UI tree does not own ───────────────────
	// These live in web/index.html and are shown by page script after wasm
	// mounts, so they persist and were the last user-facing English the language
	// setting could not reach. Relabelled from Go (index.html's JS is frozen).
	"boot.install":        "Install CashFlux",
	"boot.iosHintDismiss": "Dismiss install hint",
	"boot.iosHint":        "On iPhone or iPad: tap Share, then 'Add to Home Screen' to install CashFlux.",

	// ── C362: built-in preset copy that gets persisted into a user's spec ────
	"widget.spotlightHeading": "This month",
	"widget.spotlightCaption": "Net {{cashflow_net|signed}} · {{floor(savings_rate)|number}}% saved",

	// ── C549: merge into a category that does not exist yet ─────────────────
	"categories.mergeIntoNew":            "＋ New category…",
	"categories.mergeNewNameLabel":       "Name for the new category",
	"categories.mergeNewNamePlaceholder": "e.g. Groceries",

	// ── C372: a rule's durable record, beside its live match count ──────────
	// The live count says what the rule would catch today; this says what it has
	// actually done. %d = lifetime matches, %s = the date it last fired.
	"rules.hitsEver":    "%d filed all-time",
	"rules.hitsLastRun": "Last filed a transaction on %s",
	"rules.hitsNoDate":  "Filed transactions before this app started recording when",

	// ── C373: the three actions the benchmark audit found missing ───────────
	// All three are apply-once: a rule fills a gap, it never reverses a person's
	// explicit choice, so there is no "un-assign" or "un-review" to offer.
	"rules.memberFieldLabel":   "Assign to",
	"rules.memberNone":         "Don't assign anyone",
	"rules.reviewedFieldLabel": "Mark reviewed (skip the review inbox)",
	"rules.excludeFieldLabel":  "Exclude from reports (still counts in balances)",
	"rules.memberMeta":         "assigns %s",
	"rules.reviewedMeta":       "marks reviewed",
	"rules.excludeMeta":        "excludes from reports",

	// ── C377/C378: allocation dimensions and what the funds cost ────────────
	"investments.bySector": "By sector",
	"investments.byRegion": "By region",
	// %s = the annual cost, %s = the value-weighted ratio as a percentage.
	"investments.feeDrag": "Fund fees cost about %s a year at current value (%s%% weighted).",
	// %d = the share of portfolio value that actually carries a recorded ratio.
	"investments.feeDragPartial": "Based on the %d%% of your portfolio with a recorded expense ratio.",

	// ── C379: drift against the household's own target allocation ───────────
	// The virtual line is not a disclaimer bolted on — it is what the feature IS.
	// CashFlux has no brokerage connection and will never place a trade.
	"investments.rebalanceTitle":   "Drift from your targets",
	"investments.rebalanceVirtual": "Nothing here moves money. These are the amounts that would bring each class back to the share you picked — CashFlux never places trades.",
	// %s = the money that would change hands, %s = the largest drift as a percent.
	"investments.rebalanceTotal": "About %s would move in total; the furthest class is %s%% off target.",
	"investments.driftOnTarget":  "On target",
	"investments.driftAdd":       "Would add about %s",
	"investments.driftTrim":      "Would move about %s out",

	// ── C400: the distinction both goal figures exist to draw ───────────────
	"goals.savedVsSetAside": "Saved = money you've moved into a linked account. Set aside = money earmarked for this goal that stays where it is — nothing moves.",

	// ── C401: resolve a "Needs a plan" goal without opening it ──────────────
	// %s = the date this goal could actually be met by at its fair share of free
	// cash. Naming the date is the point — "push it out" without one is not an
	// action, it is a shrug.
	"goals.retargetAction": "Move the deadline to %s",
	"goals.retargetTitle":  "The earliest date this goal could be met at its fair share of your free monthly cash",
	// %s = goal name, %s = the old date, %s = the new one.
	"goals.retargetedNotice": "%s: deadline moved from %s to %s.",
	"goals.archiveTitle":     "Stop tracking this goal — it stays in your history",

	// ── C409: resolve an alert from the alert ───────────────────────────────
	"notifications.resolveMarkPaid":    "Mark paid",
	"notifications.resolveMarkUpdated": "Mark updated",
	"notifications.resolveMarkDone":    "Mark done",
	"settings.alert.taskReminder":      "To-do reminders",

	// C402 — bulk selection on the to-do list.
	"action.undo":                "Undo",
	"todo.selectTask":            "Select this to-do",
	"todo.bulkSelectMode":        "Select several",
	"todo.bulkBarLabel":          "Bulk actions",
	"todo.bulkSelected":          "%d selected",
	"todo.bulkSelectAll":         "Select all on this page",
	"todo.bulkAssignPlaceholder": "Assign to…",
	"todo.bulkUnassign":          "No one",
	"todo.bulkAssign":            "Assign",
	"todo.bulkDuePlaceholder":    "Move due date…",
	"todo.bulkDueToday":          "Today",
	"todo.bulkDueTomorrow":       "Tomorrow",
	"todo.bulkDueNextWeek":       "Next week",
	"todo.bulkDueNextMonth":      "Next month",
	"todo.bulkDuePushWeek":       "Push a week",
	"todo.bulkDueClear":          "Clear the due date",
	"todo.bulkReschedule":        "Reschedule",
	"todo.bulkComplete":          "Mark done",
	"todo.bulkDelete":            "Delete",
	"todo.bulkClear":             "Clear selection",
	"todo.bulkExit":              "Done selecting",
	"todo.bulkUpdated":           "%d to-dos updated.",
	"todo.bulkCompleted":         "%d to-dos marked done.",
	"todo.bulkDeleted":           "%d to-dos deleted.",
	"todo.bulkUndone":            "Put %d to-dos back.",
	"todo.bulkNoChange":          "Those to-dos already match — nothing to change.",
	"todo.bulkNoneOpen":          "Everything selected is already done.",
	"todo.bulkDeleteConfirm":     "Delete %d to-dos, including any sub-tasks? You can undo this.",

	// C405 — ready-made automations + two more checklist templates.
	"wfpreset.title":                  "Ready-made automations",
	"wfpreset.lede":                   "Complete automations you can add in one click, then edit like any other.",
	"wfpreset.add":                    "Add this",
	"wfpreset.addAgain":               "Added — add another",
	"wfpreset.added":                  "Added the \"%s\" automation. It is running.",
	"wfpreset.priceChange.name":       "Tell me when a subscription's price changes",
	"wfpreset.priceChange.desc":       "When a charge lands more than 15% above what that merchant normally bills, make a to-do to check it. Merchants with too little history are left alone.",
	"wfpreset.overdueBill.name":       "Turn a bill that comes due into a to-do",
	"wfpreset.overdueBill.desc":       "When a bill reaches its due date, add a to-do to pay it, so it leaves the calendar and joins the list you actually work.",
	"wfpreset.bigCharge.name":         "Flag big charges for review",
	"wfpreset.bigCharge.desc":         "When a charge over $500 is added, tag it for review and say so. Edit the amount after adding.",
	"wfpreset.monthlyRecon.name":      "Reconcile every month",
	"wfpreset.monthlyRecon.desc":      "Once a month, sort what the rules can sort, then add a to-do to match balances against statements.",
	"wfpreset.quarterlyAccounts.name": "Update the accounts that do not sync",
	"wfpreset.quarterlyAccounts.desc": "Every quarter, add a to-do to refresh the balances the app cannot see for itself — retirement, property, loans.",
	"wfpreset.budgetOver.name":        "Make a to-do for every budget that goes over",
	"wfpreset.budgetOver.desc":        "When a budget passes its limit, create a to-do for it so going over is something you decide about, not something you notice later.",
	"todo.automations":                "Set up an automation…",
	"todo.checklistSubAudit":          "Subscription & insurance audit",
	"todo.checklistDebt":              "Debt check-in",
	"todo.tmplSubAudit":               "Subscription & insurance audit — %s",
	"todo.tmplSubList":                "List every recurring charge",
	"todo.tmplSubUnused":              "Mark the ones nobody used this quarter",
	"todo.tmplSubPrices":              "Check which prices went up",
	"todo.tmplSubInsurance":           "Re-quote insurance",
	"todo.tmplSubCancel":              "Cancel or downgrade what did not survive the list",
	"todo.tmplDebt":                   "Debt check-in — %s",
	"todo.tmplDebtBalances":           "Write down every balance owed",
	"todo.tmplDebtRates":              "Check the interest rate on each",
	"todo.tmplDebtExtra":              "Decide what extra payment is affordable",
	"todo.tmplDebtPlan":               "Point the extra at one debt and schedule it",

	// C404 — saved views + the single adaptive toolbar.
	"todo.views":             "Views",
	"todo.viewsLabel":        "Saved views",
	"todo.viewSave":          "Save this view…",
	"todo.viewSavePrompt":    "Name this view",
	"todo.viewSaved":         "Saved the view \"%s\".",
	"todo.viewDelete":        "Delete this saved view",
	"todo.viewDeleteConfirm": "Delete the saved view \"%s\"? Your to-dos are not affected.",
	"todo.viewDeleted":       "Deleted the view \"%s\".",
	"todo.viewNeedsName":     "A saved view needs a name.",
	"todo.viewTooMany":       "You can save up to %d views. Delete one to make room.",
	"todo.filters":           "Filters",
	"todo.filtersTitle":      "Filter and sort to-dos",
	"todo.filtersActiveAria": "Filters — %d active",
	"todo.clearFilters":      "Clear filters",
	"todo.removeFilter":      "Remove this filter",
	"todo.chipPriority":      "Priority: %s",
	"todo.chipLink":          "Linked to: %s",
	"todo.chipHideDone":      "Done tasks hidden",

	// C398 — Compare: state the eligibility rule, and show the funding-order trade.
	"goalcompare.eligibleRule":          "You can compare any goal that is measured in money, has a target amount, and is not archived.",
	"goalcompare.exclLead":              "Left out: %s.",
	"goalcompare.exclNotFinancial":      "%d not measured in money",
	"goalcompare.exclNoTarget":          "%d with no target amount",
	"goalcompare.exclArchived":          "%d archived",
	"goalcompare.orderTitle":            "If you funded one first",
	"goalcompare.orderLede":             "Same %s a month, given to one goal at a time instead of split between them.",
	"goalcompare.orderAFirst":           "%s first",
	"goalcompare.orderBFirst":           "%s first",
	"goalcompare.orderMonths":           "%d months",
	"goalcompare.orderOneMonth":         "1 month",
	"goalcompare.orderAlready":          "Already there",
	"goalcompare.orderNever":            "Not on this plan",
	"goalcompare.orderMoot":             "Funding order does not change either date — the monthly plans already fit alongside each other.",
	"goalcompare.orderNeedsPlans":       "Both goals need a monthly plan before funding order can be compared.",
	"goalcompare.orderNeedsOneCurrency": "Funding order can only be compared when both goals use the same currency.",

	// C399 — contribution history: planned vs actual.
	"goalhist.title":          "Funded each month",
	"goalhist.legend":         "The marker on each bar is the %s monthly plan.",
	"goalhist.readOnPlan":     "Met the plan every month — %d of them.",
	"goalhist.readBehind":     "%d months came in short, %s under plan in total.",
	"goalhist.readNoPlan":     "%s contributed over the last year.",
	"goalhist.barAria":        "%s: %s contributed",
	"goalhist.barAriaPlanned": "%s: %s contributed against a %s plan",

	// C383 — report window + comparison-period pickers.
	"rptrange.label":         "Report window",
	"rptrange.window":        "Window",
	"rptrange.compare":       "Compare with",
	"rptrange.trailing12":    "Last 12 months",
	"rptrange.lastYear":      "Last calendar year",
	"rptrange.ytd":           "This year so far",
	"rptrange.trailing6":     "Last 6 months",
	"rptrange.trailing3":     "Last 3 months",
	"rptrange.custom":        "Choose months…",
	"rptrange.cmpLastYear":   "The same months last year",
	"rptrange.cmpPrior":      "The period just before",
	"rptrange.cmpNone":       "Nothing",
	"rptrange.from":          "From",
	"rptrange.to":            "To",
	"rptrange.inclusiveHint": "Both months are included.",
	"rptrange.span":          "%s – %s",
	"rptrange.showing":       "Showing %s.",
	"rptrange.showingVs":     "Showing %s, compared with %s.",

	// C385 — the per-section "How this is computed" drawer. Benchmarks state
	// their source in the same line as the value: splitting the two is how an
	// unsourced-looking claim happens even when the source exists.
	// C384 — the monthly grid export.
	"reports.monthlyFlow": "Month-by-month cash flow",

	// C386 — report drills carry the report's own window into the ledger.
	"rpta.drillTitle": "Open these in the transaction list, %s to %s",

	// C408 — the evidence behind an alert, and the rule-test preview.
	"notifWhy.label":     "Why this fired",
	"notifWhy.trigger":   "What happened",
	"notifWhy.threshold": "Your setting",
	"notifWhy.observed":  "What we saw",
	"notifWhy.about":     "About",
	"notifWhy.open":      "Open it",
	"why.billDue":        "A bill reached its due window.",
	"why.billLead":       "Alerts start %d days ahead.",
	"why.billDaysUntil":  "Due in %d days.",
	"why.budget":         "Spending crossed a budget's line.",
	"why.budgetLimit":    "Limit %s.",
	"why.budgetSpent":    "Spent %s, which is %d%% of the limit.",
	"why.stale":          "An account's balance has not been confirmed recently.",
	"why.staleDays":      "Last confirmed %d days ago.",
	"why.lowBalance":     "An account dropped below your floor.",
	"why.lowFloor":       "Floor %d.",
	"why.lowNow":         "Balance %d.",
	"why.task":           "A to-do reached its reminder time.",
	"why.taskLead":       "Reminds %d days before it is due.",
	"why.taskDays":       "Due in %d days.",
	"why.unusual":        "A charge was far above what this merchant normally bills.",
	"why.unusualTypical": "Normally %d.",
	"why.unusualNow":     "This charge %d.",

	"settings.alert.test":     "Test this rule",
	"settings.alert.testAria": "Test this rule: %s",
	"settings.alert.testNone": "Nothing would fire right now.",
	"settings.alert.testOne":  "1 alert would fire: %s",
	"settings.alert.testMany": "%d alerts would fire, starting with: %s",

	// C407 — per-member notification routing.
	"settings.alert.memberLabel":    "Belongs to",
	"settings.alert.memberAll":      "Everyone",
	"settings.alert.memberAria":     "Who this alert belongs to: %s",
	"notifications.memberAll":       "Everyone's alerts",
	"notifications.memberLensLabel": "Show whose alerts",
	"notifications.memberChipTitle": "This alert belongs to %s",
	"notifications.memberGone":      "Unassigned",

	// C381 — where the account goes next.
	"accountsFwd.title":    "Next 90 days",
	"accountsFwd.negative": "Runs out on %s — as low as %s",
	"accountsFwd.low":      "Dips to %s on %s, ending around %s",
	"accountsFwd.rising":   "Stays above today's balance, ending around %s",
	"accountsFwd.assumes":  "Assumes your %d scheduled flows keep running as they are.",

	// C376 — paste a brokerage export instead of typing every position.
	"holdingImport.open":         "Paste positions from a brokerage",
	"holdingImport.title":        "Import positions",
	"holdingImport.account":      "Into which account",
	"holdingImport.pickAccount":  "Choose an account…",
	"holdingImport.pasteLabel":   "Paste your positions",
	"holdingImport.placeholder":  "Paste a CSV export, or copy the rows straight out of a spreadsheet.",
	"holdingImport.hint":         "Paste the table and you will see exactly what would change before anything is saved.",
	"holdingImport.summary":      "%d new, %d updated, %d skipped.",
	"holdingImport.done":         "Imported: %d added, %d updated, %d skipped.",
	"holdingImport.commit":       "Import %d positions",
	"holdingImport.mapped":       "Columns read as: %s",
	"holdingImport.mapNone":      "no columns recognised",
	"holdingImport.colAction":    "What happens",
	"holdingImport.colPosition":  "Position",
	"holdingImport.colShares":    "Shares",
	"holdingImport.colNote":      "Note",
	"holdingImport.actionAdd":    "New",
	"holdingImport.actionUpdate": "Update",
	"holdingImport.actionSkip":   "Skipped",
	"holdingImport.sharesFrom":   "%s to %s",
	"holdingImport.errParse":     "That does not read as a table: %s",
	"holdingImport.errNoRows":    "Include the header row and at least one position.",
	"holdingImport.errProfile":   "%s",
	"holdingImport.fTicker":      "ticker",
	"holdingImport.fName":        "name",
	"holdingImport.fShares":      "shares",
	"holdingImport.fCost":        "cost basis",
	"holdingImport.fPrice":       "price",
	"holdingImport.fClass":       "asset class",
	"holdingImport.fSector":      "sector",
	"holdingImport.fRegion":      "region",

	// C380 — a user-imported comparison series. No market feed; the user brings
	// the numbers, so the copy says so rather than implying one is coming.
	"bench.add":             "Compare against a benchmark",
	"bench.replace":         "Replace %s",
	"bench.remove":          "Remove the comparison",
	"bench.nameLabel":       "What is this series",
	"bench.namePlaceholder": "S&P 500, my old 401k, …",
	"bench.pasteLabel":      "Paste the series",
	"bench.placeholder":     "Two columns: a date and a value. Paste a CSV, or copy the rows out of a spreadsheet.",
	"bench.import":          "Use this series",
	"bench.localOnly":       "CashFlux has no market-data feed and never fetches prices. Export a series from wherever you already get it and paste it here — it stays on this device.",
	"bench.imported":        "Comparing against %s — %d points, %s to %s.",
	"bench.removed":         "Comparison removed.",
	"bench.errName":         "Give the series a name, so the comparison says what it is comparing against.",
	"bench.errParse":        "That does not read as a dated series. Two columns: a date and a value.",
	"bench.ahead":           "You are %s ahead of %s over this window.",
	"bench.behind":          "You are %s behind %s over this window.",
	"bench.detail":          "Your portfolio %s, %s %s, over %d points.",
	"bench.skipped":         "%d points of this window fall outside the series you imported, so they are not compared.",
	"bench.noOverlap":       "%s does not cover this window, so there is nothing to compare yet.",

	// E3 — where the app's own numbers disagree with each other.
	"contra.title": "These figures disagree",
	"contra.count": "%d found",
	"contra.look":  "Go look",
	"contra.more":  "Show %d more",
	"contra.less":  "Show fewer",

	// LF-8 — unfinished work, as distinct from the integrity findings beside it.
	"health.hygieneLead":            "Worth finishing:",
	"health.hygiene.uncategorized":  "%d uncategorized",
	"health.hygiene.stale-accounts": "%d stale balances",
	"health.hygiene.unreconciled":   "%d never reconciled",

	// LF-4 — the command palette can find things, not only run commands.
	"palette.groupFound": "Found in your data",

	// LF-7 — bills and recurring cash flows as a month grid.
	"rhyCal.open":   "Show the month",
	"rhyCal.close":  "Hide the month",
	"rhyCal.today":  "Back to this month",
	"rhyCal.label":  "Bills and recurring payments by month",
	"rhyCal.legend": "Each day shows what is due. Dimmed means settled; highlighted means overdue.",

	// LF-10 — a detected recurring pattern is also a categorization pattern.
	"rhythm.alsoFile":  "Also file these automatically",
	"rhythm.ruleHint":  "%d of this merchant's %d charges land in one category — a rule would file the rest for you",
	"rhythm.ruleAdded": "Charges from %s will file automatically from now on (%d matched so far).",

	// LF-1 — the palette as a launcher, not only a navigator.
	"cmd.addTransaction": "Add a transaction",
	"cmd.addTask":        "Add a to-do",
	"cmd.addAccount":     "Add an account",
	"cmd.addBudget":      "Add a budget",
	"cmd.addGoal":        "Add a goal",
	"palette.groupViews": "Saved views",

	// LF-2 — a backup you can safely put somewhere else.
	"cmd.backupEncrypted":       "Back up everything, encrypted",
	"backup.passphrasePrompt":   "Choose a passphrase for this backup. Write it down — without it the file cannot be opened, by you or anyone else.",
	"backup.passphraseConfirm":  "Type the passphrase again",
	"backup.passphraseShort":    "Use at least %d characters. This file holds everything, and it will outlive the device.",
	"backup.passphraseMismatch": "Those did not match. Nothing was saved.",
	"backup.encryptErr":         "The backup could not be encrypted, so nothing was saved.",
	"backup.encryptedDone":      "Encrypted backup saved. Keep the passphrase somewhere separate from the file.",
	"backup.unlockPrompt":       "Passphrase for this backup",
	"backup.unlockFailed":       "That did not open the file — wrong passphrase, or the file is damaged. Nothing was changed.",

	// DP1 — overdue rows are not "next 30 days".
	"rhythm.agendaOverdue": "Overdue",

	// FP-T1a/T1b — the retirement card on /planning. Real dollars lead; the
	// assumptions are stated back under the figures they produced.
	"retire.title":           "Retirement",
	"retire.basis":           "Projected from %d retirement account(s), %s today.",
	"retire.noAccounts":      "Mark an account as Retirement and this will project it forward.",
	"retire.needsHorizon":    "Set how many years until you retire to see a projection.",
	"retire.years":           "Years until you retire",
	"retire.contribution":    "Added each year (%s)",
	"retire.spend":           "Spending each year in retirement (%s)",
	"retire.return":          "Expected return (%)",
	"retire.inflation":       "Expected inflation (%)",
	"retire.potReal":         "About %s in today's money, in %d years",
	"retire.potNominal":      "%s in the dollars of that year",
	"retire.split":           "%s of that is what you put in; %s is growth.",
	"retire.lastsYears":      "At that spending it lasts about %d years.",
	"retire.lastsBeyond":     "At that spending it outlasts %d years, ending around %s in today's money.",
	"retire.fireYears":       "You would need about %s to stop working. At this rate, roughly %d years.",
	"retire.fireUnreachable": "You would need about %s to stop working — the current rate does not get there.",
	"retire.fireSource":      "Uses a %.0f%% withdrawal rate — a widely-used rule of thumb about one historical period, not a guarantee.",
	"retire.assumptions":     "Assumes %.1f%% return and %.1f%% inflation — %.2f%% after inflation.",

	// FP-T1c — the return under the balance chart. Both returns ship because they
	// answer different questions; the gap between them is what timing cost.
	"invret.why":      "The line above is a balance — it rises when the market rises and when you pay in. These are the returns.",
	"invret.twr":      "The investments returned %.1f%% a year.",
	"invret.irr":      "You returned %.1f%% a year, counting when you paid in.",
	"invret.gapCost":  "The timing of your contributions cost about %.1f points a year.",
	"invret.gapGain":  "The timing of your contributions gained about %.1f points a year.",
	"invret.basis":    "Over %d days, from %d valuations and %d transfers in or out.",
	"invret.period":       "The investments returned %.1f%% over these %d days.",
	"invret.noYearlyRate": "Too short to state as a yearly rate — annualizing under %d days multiplies the noise as much as the return.",
	"invret.tooShort": "There is not enough history yet — a return needs at least %d days to mean anything.",

	// FP-T2d — long-horizon figures restated in money the reader can price. One
	// household inflation assumption feeds every surface that discounts.
	"planning.forecastReal":  "About %s in today's money, after a year of %.1f%% inflation.",
	"retire.inflationShared": "Inflation is one household setting — changing it here also changes the forecast.",

	// FP-T2b — the per-payee shape beside the per-payee total.
	"rpta.payeeSparkAlt": "Monthly spending at %s over the year",

	// FP-T2c — the holding price editor and how old the price is.
	"investments.updatePrice":     "Update price",
	"investments.priceLabelPer":   "Price per share (%s)",
	"investments.priceAsOfLabel":  "Price as of",
	"investments.pricedOn":        "priced %s",
	"investments.priceNoDate":     "price date not recorded",

	// FP-T2a — the amortization schedule table.
	"loans.scheduleShow":      "Show all %d payments",
	"loans.scheduleHide":      "Hide the payment schedule",
	"loans.scheduleNoteBase":  "Every scheduled payment, and how much of each one goes to interest rather than to the balance.",
	"loans.scheduleNoteExtra": "Every payment with your extra amount included — this is the accelerated schedule, not the original one.",
	"loans.colNo":             "#",
	"loans.colDate":           "Date",
	"loans.colPayment":        "Payment",
	"loans.colPrincipal":      "Principal",
	"loans.colInterest":       "Interest",
	"loans.colBalance":        "Balance left",

	// FP-T1d — purchase history (tax lots) and recording a sale.
	"lots.show":        "Purchase history (%d)",
	"lots.hide":        "Hide purchase history",
	"lots.why":         "Shares bought on different days at different prices are taxed differently. Recording what you paid, and when, is what lets a sale report a real gain.",
	"lots.none":        "No purchases recorded yet. Add one and a sale can report what it actually earned.",
	"lots.partial":     "These purchases account for %s of your %s shares — a sale cannot be costed until they cover the position.",
	"lots.date":        "Bought on",
	"lots.shares":      "Shares",
	"lots.cost":        "What they cost (%s)",
	"lots.add":         "Add purchase",
	"lots.short":       "held under a year",
	"lots.long":        "held over a year",
	"lots.removeAria":  "Remove the purchase from %s",
	"lots.errDate":     "Enter the date you bought them — it decides the tax rate.",
	"lots.errShares":   "Enter how many shares you bought.",
	"lots.errCost":     "Enter what the shares cost.",
	"sell.recordSale":  "Record a sale",
	"sell.shares":      "Shares sold",
	"sell.proceeds":    "What you received (%s)",
	"sell.date":        "Sold on",
	"sell.basisMethod": "Which shares were sold",
	"sell.methodFifo":  "Oldest first (the usual assumption)",
	"sell.methodHifo":  "Most expensive first (smallest gain)",
	"sell.methodLifo":  "Newest first",
	"sell.gain":        "This sale realizes %s, against a cost of %s.",
	"sell.split":       "%s of it is short-term, %s long-term.",
	"sell.method":      "Short-term gains are usually taxed at a higher rate than long-term ones.",
	"sell.noLots":      "No purchase history for this position, so there is no cost to subtract. Add what you paid above and the gain can be worked out.",
	"sell.uncovered":   "Your purchases account for %s of the %s shares you hold. Record the rest before selling — otherwise the sale has no cost to work from and would leave the position wrong.",
	"sell.incomplete":  "Fill in the shares, the amount received and the date to see what this sale realizes.",
	"sell.record":      "Record this sale",
	"sell.recorded":    "Recorded the sale of %s — %s realized.",

	// FP-T1f — what a position has PAID OUT, as distinct from what it grew.
	"income.add":           "Record a dividend",
	"income.amount":        "Amount received (%s)",
	"income.date":          "Paid on",
	"income.note":          "Note",
	"income.notePlaceholder": "Quarterly dividend",
	"income.record":        "Record it",
	"income.defaultDesc":   "Dividend from %s",
	"income.recorded":      "Recorded %s from %s.",
	"income.paid":          "Has paid you %s across %d payments.",
	"income.paidOne":       "Has paid you %s in one payment.",
	"income.paidYield":     "Has paid you %s across %d payments — %.2f%% of what the shares cost.",
	"income.paidYieldOne":  "Has paid you %s in one payment — %.2f%% of what the shares cost.",
	"income.annual":        "That is about %.2f%% a year on what you paid.",
	"income.errAmount":     "Enter how much you received.",
	"income.errDate":       "Enter the date it was paid — income is taxed in the year it arrives.",

	"categories.taxLine":     "Schedule C line (used when this category is deductible)",
	"categories.taxLineNone": "Not assigned yet",

	// FP-T1e — business + tax depth: Schedule C grouping, realized gains,
	// estimated quarterly tax.
	"tax.secTitle":        "Business and tax",
	"tax.secSub":          "What your deductible spending looks like on the form, what your sales realized, and what to send this quarter.",
	"tax.schedTitle":      "Deductible spending by tax line",
	"tax.schedNote":       "Grouped the way Schedule C groups it, in form order, so it can be transcribed line by line.",
	"tax.schedEmpty":      "No deductible spending in this period. Mark a category deductible and give it a tax line to see it here.",
	"tax.schedUnassigned": "%s is deductible but has no tax line yet: %s. It is not counted above — unclassified spending is not \"other expenses\".",
	"tax.colLine":         "Line",
	"tax.colDesc":         "Description",
	"tax.colAmount":       "Amount",
	"tax.colFrom":         "From",
	"tax.total":           "Total",
	"tax.export":          "Export as CSV",
	"tax.gainsTitle":      "Realized gains and losses",
	"tax.gainsEmpty":      "No sales recorded in this period. Recording a sale on Investments works out its gain.",
	"tax.gainsMixed":      "These sales used more than one way of choosing which shares were sold (%s), so the totals cannot be reproduced from a single rule.",
	"tax.shortTerm":       "Short-term",
	"tax.longTerm":        "Long-term",
	"tax.proceeds":        "Proceeds",
	"tax.basis":           "Cost",
	"tax.lossAll":         "A net loss of %s, which can usually offset ordinary income this year.",
	"tax.lossCapped":      "%s of the loss can usually offset ordinary income this year; %s carries forward.",
	"tax.colSold":         "Sold",
	"tax.colWhat":         "What",
	"tax.colProceeds":     "Received",
	"tax.colBasis":        "Cost",
	"tax.colGain":         "Gain",
	"tax.estTitle":        "What to send this quarter",
	"tax.rateLabel":       "Your effective rate (%)",
	"tax.priorTaxLabel":   "Tax you owed last year (%s)",
	"tax.paidLabel":       "Paid so far this year (%s)",
	"tax.estNeedRate":     "Enter your effective rate — income tax plus self-employment tax — and this can be worked out. It is not guessed, because it depends on your filing status and other income.",
	"tax.estNeedIncome":   "No net business income in this period yet, so there is nothing to estimate from.",
	"tax.estDue":          "Send about %s — quarter %d, due %s.",
	"tax.estAhead":        "You are about %s ahead — quarter %d, due %s.",
	"tax.estHarbor":       "Paying %s for the year (%.0f%% of last year's tax) avoids an underpayment penalty whatever this year does.",
	"tax.estNoHarbor":     "Enter last year's tax and this can also show the amount that avoids a penalty regardless of how this year turns out.",
	"tax.estBasis":        "From %s of net business income so far, projected to %s of tax for the year; aiming at %s.",
	"tax.estCaveat":       "An estimate from your own rate, not tax advice. The quarters are not calendar quarters: they end in March, May, August and December.",

	// FP-T3b — committed vs chosen spending, and budget variance.
	"mix.secTitle":    "Commitments and choices",
	"mix.secSub":      "How much of this period you could have done something about, and how far your budgets landed from plan.",
	"mix.splitTitle":  "What was committed, what was chosen",
	"mix.empty":       "No spending in this period to split.",
	"mix.share":       "%.0f%% of what you spent was a choice — %s.",
	"mix.fixed":       "Commitments",
	"mix.nonMonthly":  "Irregular",
	"mix.flex":        "Chosen",
	"mix.committed":   "Spoken for",
	"mix.note":        "Commitments are the things you cannot change this month; irregular costs are real but do not repeat monthly. What is left is where changing anything is possible.",
	"mix.unknown":     "%s has no category, so it is counted in the total but not in any of these. Uncategorized spending is not assumed to be a choice.",
	"mix.varTitle":    "How far budgets landed from plan",
	"mix.varEmpty":    "No budgets with a limit in this period.",
	"mix.varHeadline": "%s over on some budgets, %s under on others. These are shown apart because being over on one and under on another is not the same as being on plan.",
	"mix.colBudget":   "Budget",
	"mix.colPlanned":  "Planned",
	"mix.colActual":   "Actual",
	"mix.colDiff":     "Difference",
	"mix.over":        "%s over",
	"mix.under":       "%s under",
	"mix.onPlan":      "on plan",

	// FP-T3c — paying fortnightly, and combining debts. Both panels must be able
	// to say NO; a comparison that only reports good news is an advertisement.
	"accel.title":          "Two ways to pay this off faster",
	"accel.sub":            "Both of these are advice people are given constantly and rarely given the arithmetic for. Here is what they would actually do to your debts.",
	"accel.biweeklyTitle":  "Paying every two weeks",
	"accel.whichDebt":      "Which debt",
	"accel.termLabel":      "Term (months)",
	"accel.biweeklySaving": "Saves about %s in interest and finishes %d months sooner.",
	"accel.biweeklyCost":   "That means paying %s every two weeks — %s more each year than you pay now.",
	"accel.biweeklyWhy":    "The gain comes from paying more, not from paying more often: twenty-six half-payments is thirteen monthly payments, not twelve. If a thirteenth payment is not affordable, neither is this plan.",
	"accel.noGain":         "At this rate and term, paying fortnightly changes nothing worth acting on.",
	"accel.noDebts":        "No debts with a balance to model.",
	"accel.cannotModel":    "Not enough to work from — a debt needs a balance and a term before this can be compared.",
	"accel.consolTitle":    "Combining these debts into one",
	"accel.needTwo":        "Combining needs at least two debts with a balance.",
	"accel.blended":        "You are currently paying about %.1f%% across these debts, weighted by what you owe. That is the rate an offer has to beat — not your worst one.",
	"accel.newApr":         "New rate (%)",
	"accel.newTerm":        "New term (months)",
	"accel.feeLabel":       "Origination fee (%)",
	"accel.consolSaves":    "Saves about %s in interest.",
	"accel.consolCosts":    "Costs about %s MORE in interest.",
	"accel.payMore":        "But %s more each month — %s instead of what you pay now.",
	"accel.payLess":        "And %s less each month — %s instead of what you pay now.",
	"accel.feeFinanced":    "Includes a %s fee added to what you borrow, which is how these are normally charged.",
	"accel.consolBasis":    "Compared against clearing them separately, which takes %d months; the new loan takes %d.",
	"accel.termDriven":     "Most of that comes from the shorter term, not the rate — %d months instead of %d. You would save something similar by paying your current debts down that fast, and the higher monthly payment is what buys it.",
	"accel.unmodelled":     "Left out because they have no monthly payment recorded, or a payment that never clears the balance: %s. The comparison above is incomplete without them.",

	// FP-T3a — a plan that is more than a straight line.
	"plans.growthLabel": "Monthly amount grows each year (%)",
	"plans.returnLabel": "Balance earns each year (%)",
	"plans.ratesNote":   "A raise lands on its anniversary rather than being spread across the year, and a negative balance earns nothing — you pay interest on an overdraft, you do not collect it.",

	// FP-T3d — the drawdown as a probability, with its method stated.
	"retire.mc":       "Across two thousand simulated futures, the money lasted %.0f%% of the time over %d years.",
	"retire.mcFail":   "In the runs that ran out, the earliest was year %d.",
	"retire.mcMethod": "Method: %d runs, returns drawn from a normal distribution around your expected return, fixed seed %d so this number is the same every time and can be checked. Real markets have more extreme years than a normal distribution does.",

	// DP-F5a/b — the board's empty lanes, and where a calendar-seeded task lands.
	"todoboard.addHere":     "+ Add one here",
	"todoboard.addHereAria": "Add a task in this column",
	"todo.schedulingFor":    "Scheduling for %s",

	// DP-F5c — say what "Save as template" snapshots.
	"txnTemplates.saveHint": "Saves what you have filled in above",

	"method.drawerLabel":  "How this is computed",
	"method.howTitle":     "The calculation",
	"method.benchTitle":   "What it is measured against",
	"method.exclTitle":    "What is left out",
	"method.sourcePrefix": "Source: %s",

	"method.f.health":         "Financial health is a weighted score over savings rate, budget adherence, emergency-fund months, debt load and account freshness.",
	"method.f.savingsRate":    "Savings rate = (income - spending) / income, over the window shown.",
	"method.f.income":         "Income = every positive transaction in scope, converted to your base currency at the stored rate.",
	"method.f.expense":        "Spending = every negative transaction in scope, as a positive magnitude.",
	"method.f.net":            "Net = income minus spending. It is not the change in your balances; a transfer moves money without being either.",
	"method.f.monthBuckets":   "Each month is a whole calendar month, counted from its first day to the first day of the next.",
	"method.f.inProgress":     "The newest month is marked in progress when today falls inside it - its total is partial by definition.",
	"method.f.catTotal":       "A category total sums the transactions filed to it, plus its sub-categories when rollup is on.",
	"method.f.catShare":       "Share = this category divided by total spending in the window.",
	"method.f.catYoY":         "The change compares this window against the comparison period you chose above.",
	"method.f.payeeTotal":     "A payee total groups by the payee name, falling back to the description when no payee is set.",
	"method.f.goalCoverage":   "Coverage = money saved into the goal plus money earmarked for it, against its target.",
	"method.f.goalProjection": "The projected date accrues the goal's monthly plan from today until it reaches the target.",
	"method.f.budgetUsed":     "Used = spending filed to the budget's categories within the budget's own period.",
	"method.f.budgetPace":     "Pace compares how much of the budget is spent against how much of the period has elapsed.",
	"method.f.creditUtil":     "Utilization = balance owed divided by credit limit, per card and across all cards.",
	"method.f.creditScore":    "The credit-habits score weights utilization, on-time payments, account age and how many cards are near their limit.",
	"method.f.unusual":        "A charge is unusual when it exceeds the median of that merchant's other charges by a wide margin.",
	"method.f.subsDrift":      "A subscription's price change compares its latest charge against the median of its earlier ones.",
	"method.f.planTrim":       "A suggested trim takes the recent average for a category and projects a year of the difference.",
	"method.f.planProject":    "Projections continue the current monthly pattern forward; they are arithmetic, not a forecast of your life.",

	"method.exclTransfers":         "Transfers between your own accounts. Moving money is not earning or spending it, and counting transfers is the most common reason a total looks too big.",
	"method.exclFlagged":           "Transactions you marked as excluded from reports.",
	"method.exclFx":                "Amounts in other currencies use the exchange rate stored in Settings, not a live rate.",
	"method.exclUncategorized":     "Transactions with no category are grouped separately rather than spread across the others.",
	"method.exclPayeeBlank":        "Transactions with neither a payee nor a description cannot be grouped and are left out.",
	"method.exclArchivedGoals":     "Archived goals.",
	"method.exclNonFinancialGoals": "Goals not measured in money - habits, checklists and milestones have no amount to total.",
	"method.exclNoBudget":          "Spending in categories with no budget set.",
	"method.exclNotBureau":         "This is an estimate computed on your own data. It is not a credit-bureau score and no data leaves this device.",
	"method.exclThinHistory":       "Merchants with fewer than three earlier charges - there is no established normal to deviate from.",
	"method.exclProjectionAssumes": "Projections assume today's income, spending and plans continue unchanged.",

	"method.b.savingsRate":          "A healthy savings rate",
	"method.b.savingsRateValue":     "20% or more of income",
	"method.b.savingsRateSource":    "the widely-used 50/30/20 rule of thumb - a convention, not a law, and worth adjusting to your own situation",
	"method.b.healthBands":          "Health score bands",
	"method.b.healthBandsValue":     "80+ strong, 60-79 good, 40-59 needs work, under 40 critical",
	"method.b.ownConvention":        "a CashFlux convention, chosen so the same bands apply to every score in the app",
	"method.b.util":                 "Credit utilization",
	"method.b.utilValue":            "under 30% across all cards, and under 30% on each",
	"method.b.utilSource":           "the threshold major scoring models are widely reported to weight; treat it as a guideline, not a published formula",
	"method.b.emergency":            "Emergency fund",
	"method.b.emergencyValue":       "3 to 6 months of essential spending",
	"method.b.emergencySource":      "long-standing personal-finance guidance; the right number depends on how steady your income is",
	"notifications.resolvedDone":    "To-do marked done.",
	"notify.taskTitle":              "To-do: %s",
	"notify.taskBody":               "Due in %d days.",
	"notify.taskBodyToday":          "Due today.",
	"notify.taskBodyOverdue":        "Overdue by %d days.",
	"notifications.resolvedPaid":    "Marked paid.",
	"notifications.resolvedUpdated": "Balance confirmed as of today.",
}

func init() {
	maps.Copy(english, vSweepKeys)
}
