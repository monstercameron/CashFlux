// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"bv6g.monthclose.leftTitle":        "Unspent money",
		"bv6g.monthclose.leftItemRollover": "%s: %s left; unused money rolls into the next period",
		"bv6g.monthclose.leftItemFresh":    "%s: %s left; this budget starts fresh next period",
		"bv6g.monthclose.leftTotal":        "About %s a month remains across %d budgets",
		"bv6g.monthclose.monthlyAmount":    "About %s a month",
		"bv6g.monthclose.incomeLive":       "%s received so far of %s expected",
		"bv6g.cover.sourceBudget":          "%s — %s left (%s a month)",
		"bv6g.cover.nextMonthHint":         "For the selected next-month option, this period goes up and next period goes down by the same amount.",
		"bv6g.cover.apply":                 "Cover %s",
		"bv6g.raise.cap":                   "%s can spend %s this month (%s)",
		"bv6g.raise.amountError":           "Enter an amount above $0.",
		"bv6g.raise.apply":                 "Raise by %s",
		"bv6g.adjust.scopeLabel":           "Periods to update",
		"bv6g.adjust.thisPeriod":           "This period only",
		"bv6g.adjust.allPeriods":           "All periods",
		"bv6g.adjust.thisHint":             "Updates budgets in the period you are viewing.",
		"bv6g.adjust.allHint":              "Updates every budget period, including past periods.",
	} {
		english[key] = value
	}
}
