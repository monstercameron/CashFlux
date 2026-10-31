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
	"sm4.aiAction":        "Explain with Smart+",

	// SM-6 — the repeats detected on one account.
	"sm6.makeRecur": "Track as recurring",
	// %s = the cadence (monthly, weekly…), %s = the typical amount.
	"sm6.rowMeta": "%s · about %s",
	// %s = the charge now being tracked.
	"sm6.trackedUndo": "Now tracking %s as recurring.",

	// SM-8 — a likely duplicate charge.
	"sm8.chip":   "Possible duplicate",
	"sm8.hint":   "This may be a duplicate.",
	"sm8.detail": "Same amount and merchant as another transaction from the last few days.",

	// SM-10 — a category spiked against its own baseline.
	// %s = the category, %s = how much above its baseline.
	"sm10.chip":               "Higher than usual",
	"sm10.hint":               "About %s more than your usual %s at %s",
	"txnflag.showEvidence":    "Higher than usual. Show evidence",
	"txnflag.evidenceTitle":   "Why this was flagged",
	"txnflag.openTransaction": "Open transaction",

	// SM-12 — a suggested amount for one budget.
	// The undoable toast. %s = the budget name, %s = the new limit.
	"sm12.appliedUndo": "%s set to %s.",

	// SM-13 — a goal pace nudge (Smart+ writes the sentence).
	"sm13.action": "Ask Smart+",

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
}

func init() {
	for k, v := range smartHintKeys {
		english[k] = v
	}
}
