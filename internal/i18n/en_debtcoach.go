// SPDX-License-Identifier: MIT

package i18n

// debtCoachKeys holds English copy for the /debt coaching surfaces added in the
// interactive redesign: the Watch-outs alert tile (debtcoach rules), the strategy
// tuner, the teaching accordion, and the shared duration formatter. Kept in its own
// file (not en.go) so it doesn't touch the concurrent-WIP catalog.
var debtCoachKeys = Catalog{
	// Compact durations used across the debt readouts. %d args as noted.
	"debt.dur.now":         "now",
	"debt.dur.months":      "%d mo",       // %d = months
	"debt.dur.years":       "%d yr",       // %d = years
	"debt.dur.yearsMonths": "%d yr %d mo", // %d = years, %d = months

	// Jump-nav labels for the new sections.
	"debt.jumpWatchouts": "Watch-outs",
	"debt.jumpTune":      "Tune plan",
	"debt.jumpLearn":     "Learn",

	// Plan scope in the hero. The debt-free date is the PLAN's date (included debts
	// only), so when a debt is excluded we name what the plan clears and what's left
	// out rather than pairing the full total with the date. %s = planned balance,
	// %s = date. Overrides the older "…at current minimums" copy, which was wrong
	// once the plan can carry an extra payment.
	"debt.debtFreeBy":   "Debt-free by %s.",
	"debt.planClearsBy": "Plan clears %s by %s",
	"debt.planExcludes": "Excludes %s not in the plan — see the ladder below.",

	// --- Watch-outs tile ---
	"debt.alerts.title":         "Watch-outs",
	"debt.alerts.more":          "+ %d more like this", // %d = count of other matching debts
	"debt.alerts.allClearTitle": "Nothing to worry about",
	"debt.alerts.allClearBody": "No red flags right now — no maxed cards, no balance that's " +
		"growing, and your minimums are keeping ahead of the interest. Keep it up.",

	// Individual alerts. Each has a short title and a plain-English "why it matters".
	"debt.alert.overLimit.title": "A card is over its limit",
	"debt.alert.overLimit.body": "%s is at %.0f%% of its limit. Going over usually triggers a fee " +
		"and dents your credit score right away — bring it back under the limit first.",

	"debt.alert.underwater.title": "This balance is growing",
	"debt.alert.underwater.body": "%s's minimum payment is smaller than the interest it charges, so " +
		"the balance climbs every month no matter how long you pay. Raise the payment or move the " +
		"balance somewhere cheaper — nothing else will stop it.",

	"debt.alert.utilHigh.title": "Your cards are nearly maxed",
	"debt.alert.utilHigh.body": "You're using %s%% of your total card limit. That's a heavy drag on " +
		"your credit score, and it's the fastest thing to improve — every dollar paid down helps here.",

	"debt.alert.utilWarn.title": "Card use is creeping up",
	"debt.alert.utilWarn.body": "You're using %s%% of your total card limit. Keeping it under 30%% " +
		"is easier on your credit score, so this is a good place to aim spare cash.",

	"debt.alert.overAssets.title": "You owe more than you own",
	"debt.alert.overAssets.body": "Your debts come to %s%% of what you own. It's survivable, but " +
		"getting this back under 100%% is the milestone that turns the corner.",

	"debt.alert.highApr.title": "A high-interest debt",
	"debt.alert.highApr.body": "%s charges %.1f%% APR — the most expensive debt to carry. Paying it " +
		"off first (the avalanche method) saves you the most money overall.",

	"debt.alert.interestHeavy.title": "Payments barely dent the balance",
	"debt.alert.interestHeavy.body": "About %s%% of your monthly minimums (%s) goes straight to " +
		"interest, not the balance. A little extra each month is what actually shrinks what you owe.",

	"debt.alert.slow.title": "Decades at the minimum",
	"debt.alert.slow.body": "Paying only the minimums, you'd be clear in about %s. A modest extra " +
		"payment can cut years off that — try it in the tuner below.",

	// --- Strategy tuner ---
	"debt.tuner.title": "Tune your plan",
	"debt.tuner.hint": "Choose how to attack your debts and how much extra you can spare each month. " +
		"The ladder, debt-free date, and comparison below all update to match.",
	"debt.tuner.methodLabel":  "Payoff method",
	"debt.tuner.snowballSub":  "Smallest balance first — quick wins",
	"debt.tuner.avalancheSub": "Highest rate first — least interest",
	"debt.tuner.extraLabel":   "Extra per month (%s)", // %s = base currency code
	"debt.tuner.decrease":     "Less extra",
	"debt.tuner.increase":     "More extra",
	"debt.tuner.suggest":      "Suggest an amount",
	"debt.tuner.clear":        "Clear",
	"debt.tuner.timeToFree":   "Time to clear",
	"debt.tuner.impact":       "That's %s sooner and saves %s in interest versus paying only the minimums.",
	"debt.tuner.addExtraHint": "Add even a small extra payment to see how much time and interest it saves.",

	// Per-row accessible names — so a screen reader hears "Edit Rewards Card", not
	// one of eight identical "Edit" buttons. %s = account name.
	"debt.viewAria":         "View %s transactions",
	"debt.editAria":         "Edit %s",
	"debt.payoffToggleAria": "Include %s in the payoff plan",

	// Calculator range guard (inputs have min=0 but a typed negative still arrives).
	"debt.calcRangeError": "Enter a balance and payment above zero, and an APR of 0 or more.",

	// Shown in the strategy comparison on /debt in place of its own extra-payment
	// field (the tuner owns that control). %s = the current extra amount.
	"debt.compareAtNote": "Comparing both methods at %s extra per month — set it in Tune your plan above.",

	// --- Teaching accordion ---
	"debt.learn.title": "Understand debt",
	"debt.learn.hint":  "Short, plain-English answers to the questions that decide how fast you get free.",

	"debt.learn.methodsQ": "Snowball or avalanche — which should I pick?",
	"debt.learn.methodsA": "Both throw every spare dollar at one debt while paying minimums on the rest. " +
		"Avalanche targets your highest interest rate first, so it saves the most money. Snowball targets " +
		"your smallest balance first, so you clear whole debts sooner and keep your motivation up. If the " +
		"money difference is small, pick snowball — the plan you actually stick to is the one that works.",

	"debt.learn.trapQ": "How does debt spiral out of control?",
	"debt.learn.trapA": "Interest is charged on what you owe, every month. If your payment is smaller than " +
		"that month's interest, the balance grows even though you're paying — and next month's interest is " +
		"bigger still. This is why paying only the minimum on a high-rate card can go on for decades: most " +
		"of each payment is interest, and the balance barely moves. The way out is to pay more than the " +
		"interest, every month, on at least one debt until it's gone.",

	"debt.learn.utilQ": "Why does credit utilization matter?",
	"debt.learn.utilA": "Utilization is how much of your card limits you're using. It's one of the biggest " +
		"factors in your credit score. Under 30% is the usual guideline; over that starts to weigh on the " +
		"score, and a maxed card hurts most. Paying a card down is often the single fastest way to raise " +
		"your score — faster than time or new accounts.",

	"debt.learn.orderQ": "What order should I tackle things in?",
	"debt.learn.orderA": "A common sequence: first keep a small emergency fund (about one month of expenses) " +
		"so a surprise doesn't send you back to the cards. Then clear any debt whose balance is growing — " +
		"the ones where the minimum can't cover the interest. After that, follow your chosen method " +
		"(avalanche for least interest, snowball for momentum) on the rest. Never skip a minimum payment to " +
		"do this — a missed payment costs more than the interest you'd save.",

	"debt.learn.consolidateQ": "When does consolidating or refinancing help?",
	"debt.learn.consolidateA": "Rolling several high-rate debts into one lower-rate loan (or a 0% balance " +
		"transfer) can cut your interest and simplify payments — but only if the new rate is genuinely lower " +
		"after fees, and only if you don't run the old cards back up. Watch the transfer fee and the date a " +
		"promotional rate ends. If you'd just move the balance and keep spending, consolidating hides the " +
		"problem instead of fixing it.",
}

func init() {
	for k, v := range debtCoachKeys {
		english[k] = v
	}
}
