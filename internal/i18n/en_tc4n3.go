// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"transactions.calDayWithScheduled":    "%s. Scheduled items: %s",
		"transactions.calScheduledDueSummary": "%s is due on this day for %s.",
		"transactions.calAccountDueSummary":   "%s is due on this day for %s.",
		"transactions.calNotPostedSummary":    "%s was due on this day for %s, but it was not posted.",
	} {
		english[key] = value
	}
}
