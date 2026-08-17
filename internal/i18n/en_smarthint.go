// SPDX-License-Identifier: MIT

package i18n

// smartHintKeys holds the copy for the shared item-scoped smart hint surface
// (internal/screens/smart_rowhint.go) and for each SM feature that renders through
// it. Merged via init so this file does not touch en.go.
var smartHintKeys = Catalog{
	// The shared control.
	"smartHint.dismiss":     "Not now",
	"smartHint.dismissAria": "Hide this suggestion",

	// The shared "explain this one thing" affordance (SM-5, SM-7).
	"smartExplain.action":  "Explain with Smart+",
	"smartExplain.pending": "Thinking…",

	// SM-3 — split suggestion on one charge.
	// %d = how many past charges the shares were averaged over.
	"sm3.hint":       "You usually split this merchant.",
	"sm3.detail":     "Based on %d past charges here.",
	"sm3.action":     "Review the split",
	"sm3.aiAction":   "Suggest a split",
	"sm3.aiPending":  "Thinking…",
	"sm3.aiNoAnswer": "Smart+ didn't find a split worth proposing for this charge.",

	// SM-4 — why one budget went over. Each reason gets its own sentence; the
	// figures are supplied by internal/whyover.
	// %s = the top merchant, %s = its amount.
	"sm4.reasonOneCharge": "One merchant drove this: %s, at %s.",
	// %d = how many more charges than the period before.
	"sm4.reasonMoreOften": "You shopped here more often — %d more charges than last period.",
	// %s = this period's average charge, %s = the previous average.
	"sm4.reasonPricier": "The same number of charges, each costing more — %s on average, up from %s.",
	// %s = the projected overage.
	"sm4.reasonEarlyPace": "Not over yet, but at this rate the period lands about %s past the limit.",
	"sm4.reasonSteady":    "No single cause — the spending is spread across the period.",
	"sm4.title":           "Why this went over",
	"sm4.contributors":    "Biggest contributors",
	"sm4.action":          "See the charges",
	"sm4.aiAction":        "Explain with Smart+",

	// SM-5 — an unusual balance move on one account.
	// %s = the size of the move, %s = how it compares with this account's norm.
	"sm5.hint":     "This balance moved unusually: %s.",
	"sm5.detail":   "That's %s the size of a typical move on this account.",
	"sm5.action":   "See the charges",
	"sm5.aiAction": "Explain with Smart+",

	// SM-6 — the repeats detected on one account.
	// %d = how many repeating charges were found on this account.
	"sm6.hint":      "%d charges here repeat on a schedule.",
	"sm6.action":    "Review them",
	"sm6.title":     "Repeating charges on this account",
	"sm6.makeRecur": "Track as recurring",
	"sm6.tracked":   "Already tracked",
	// %s = the cadence (monthly, weekly…), %s = the typical amount.
	"sm6.rowMeta": "%s · about %s",
	"sm6.empty":   "Nothing on this account repeats on a schedule yet.",
	// %s = the charge now being tracked.
	"sm6.trackedUndo": "Now tracking %s as recurring.",

	// SM-8 — a likely duplicate charge.
	"sm8.chip":   "Duplicate?",
	"sm8.hint":   "This looks like a duplicate.",
	"sm8.detail": "Same amount and merchant as another charge within a few days.",
	"sm8.action": "Compare them",

	// SM-9 — an expected charge that hasn't posted.
	// %s = the merchant, %s = when it was expected.
	"sm9.hint":   "%s usually posts by now — nothing has arrived since %s.",
	"sm9.action": "Check it",

	// SM-10 — a category spiked against its own baseline.
	// %s = the category, %s = how much above its baseline.
	"sm10.chip":   "Unusual",
	"sm10.hint":   "%s is running %s above its usual.",
	"sm10.action": "See the charges",

	// SM-11 — a low-balance / overdraft forecast on one account.
	// %s = the date the balance is projected to go negative.
	"sm11.hint":   "At this burn, this account dips below zero around %s.",
	"sm11.detail": "Projected from your recent spending and the bills already scheduled.",
	"sm11.action": "See the forecast",

	// SM-12 — a suggested amount for one budget.
	// %s = the suggested limit, %d = how many months it averages.
	"sm12.hint":   "Your %d-month average here is %s.",
	"sm12.action": "Use %s",
	// The undoable toast. %s = the budget name, %s = the new limit.
	"sm12.appliedUndo": "%s set to %s.",

	// SM-13 — a goal pace nudge (Smart+ writes the sentence).
	"sm13.action":  "Ask Smart+",
	"sm13.pending": "Thinking…",

	// SM-14 — reading a date out of a typed to-do.
	// %s = the date, %s = the repeat label.
	"sm14.dueSuffix":    " · due %s",
	"sm14.repeatSuffix": " · repeats %s",
	// %s = the words the parser consumed ("on friday").
	"sm14.readFrom":   "Read from “%s”.",
	"sm14.action":     "Use this",
	"sm14.aiAction":   "Read it with Smart+",
	"sm14.aiPending":  "Thinking…",
	"sm14.aiNoAnswer": "Smart+ couldn't find a date in that.",

	// SM-15 — typing a transaction in words.
	"sm15.label":       "Type it in words",
	"sm15.placeholder": "spent 40 at whole foods yesterday",
	"sm15.action":      "Fill it in",
	"sm15.readLocal":   "Read on this device — no key needed.",
	"sm15.aiAction":    "Read it with Smart+",
	"sm15.aiPending":   "Thinking…",
	"sm15.aiNoAnswer":  "Smart+ couldn't read a transaction from that.",
	// %s = the amount as typed.
	"sm15.amountOut": "−%s out",
	"sm15.amountIn":  "+%s in",

	// SM-16 — a fee draining an otherwise-idle account.
	// %s = the yearly cost of the fee.
	"sm16.hint":   "This account is idle but still charging fees — about %s a year.",
	"sm16.action": "Review the account",
}

func init() {
	for k, v := range smartHintKeys {
		english[k] = v
	}
}
