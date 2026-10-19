// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"bv7g.cover.title":           "Cover overages",
		"bv7g.cover.action":          "Cover from another budget",
		"bv7g.cover.applyEmpty":      "Cover",
		"bv7g.monthclose.leftTitle":  "Unspent money",
		"bv7g.cover.sourceBudget":    "%s has %s free this period; about %s per month",
		"bv7g.cover.apply":           "Cover %s",
		"bv7g.adjust.thisPeriod":     "This period only",
		"bv7g.adjust.everyPeriod":    "Every period, including past periods",
		"bv7g.adjust.thisHint":       "Updates budgets in the period you are viewing.",
		"bv7g.adjust.everyHint":      "Updates every budget period, including past periods.",
		"bv7g.monthclose.step1":      "1. Overspending",
		"bv7g.monthclose.step2":      "2. Unspent money",
		"bv7g.monthclose.step3":      "3. Does the plan fit the income?",
		"bv7g.monthclose.step4":      "4. Income: expected and received",
		"bv7g.monthclose.step5":      "5. Carry top-ups into the next period",
		"bv7g.monthclose.budget":     "Budget",
		"bv7g.monthclose.left":       "Left",
		"bv7g.monthclose.monthly":    "Monthly equivalent",
		"bv7g.monthclose.nextPeriod": "Next period",
		"bv7g.monthclose.carries":    "Carries forward",
		"bv7g.monthclose.resets":     "Starts fresh",
		"bv7g.monthclose.leftTotal":  "Monthly equivalent of unspent money: %s",
	} {
		english[key] = value
	}
}
