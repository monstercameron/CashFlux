// SPDX-License-Identifier: MIT

package i18n

// glFundingKeys holds English strings for the goal-funding features: the GL1
// payday waterfall (preview-approve funding card on /goals) and the GL2 interest-
// aware ETA line on a goal card. Kept in a separate file (not en.go) so this
// change doesn't touch the concurrent-WIP catalog. Registered at init time.
var glFundingKeys = Catalog{
	// GL1 — payday waterfall preview-approve card.
	"goals.waterfallTitle": "Fund goals from your paycheck",
	// %s = income amount (e.g. "$2,400").
	"goals.waterfallBody": "Income of %s just landed. Here's a plan to fund your goals in priority order — nothing moves until you approve.",
	// One funding line: %s = goal name, %s = amount.
	"goals.waterfallLine": "%s → %s",
	// %s = leftover amount held back after every goal's quota is met.
	"goals.waterfallRemainder": "%s left over, unassigned",
	// Primary action: %s = total to fund.
	"goals.waterfallApprove": "Fund goals (%s)",
	// Secondary action.
	"goals.waterfallDismiss": "Not now",
	// Toast on approval: %s = total funded.
	"goals.waterfallDone": "Earmarked %s across your goals",
	// Accessible label for the whole card.
	"goals.waterfallAria": "Payday goal funding",

	// GL2 — interest-aware ETA line on a goal card.
	// %s = monthly contribution, %s = APY (e.g. "4.4%"), %s = target, %d = months.
	"goals.interestEta": "At %s/mo + %s you'll hit %s in %d months",
	// Contributions-vs-interest breakdown: %s = interest amount added by the yield.
	"goals.interestBreakdown": "interest adds %s",
	// Shown when the projection never reaches the target at this rate.
	"goals.interestUnreached": "This pace won't reach the target — raise the contribution.",

	// GL2 — account edit form APY field.
	"accounts.apyLabel": "Annual percentage yield (APY)",
	"accounts.apyHint":  "Optional. The yield this savings account earns, e.g. 4.4 for 4.4%. Powers interest-aware goal projections.",
}

func init() {
	for k, v := range glFundingKeys {
		english[k] = v
	}
}
