// SPDX-License-Identifier: MIT

package i18n

// trackUntrackedKeys is the copy for the "Track untracked spending" sheet: the
// bulk, per-row half of the /budgets unbudgeted strip.
//
// The wording carries two consequences the per-chip flow never stated, so it is
// deliberately blunt: tracking spending you were already doing makes a zero-based
// plan look WORSE, and pointing a category at an existing budget without raising
// that budget's limit turns money you were already spending into an overspend.
var trackUntrackedKeys = Catalog{
	"track.title": "Track untracked spending",
	// %d = the scan window in months. Naming it stops the sheet reading as "this
	// month" when it deliberately looks back a year to catch yearly bills.
	"track.intro": "Spending from the last %d months that no budget counts. Pick what to track, how much, and where it goes.",
	"track.empty": "Every category with spending is already tracked by a budget.",
	// %s = amount, %s = the date it last happened.
	"track.rowSpent":    "%s · last seen %s",
	"track.fromHistory": "amount from your history",
	"track.destNew":     "New budget",
	"track.raise":       "Raise that budget's limit",
	"track.amountAria":  "Monthly amount for %s",
	"track.destAria":    "Where to track %s",
	// Footer read. %d categories, %s spend brought in, %s added to the plan.
	"track.summaryOne":  "Tracking %d category · %s of spending · %s added to your plan",
	"track.summaryMany": "Tracking %d categories · %s of spending · %s added to your plan",
	// The zero-based bad news, stated before Apply is reachable. %s before, %s after.
	"track.toAssign": "To assign goes from %s to %s.",
	// The overspend trap, named per destination. %s = budget names.
	"track.overspendRisk": "Without raising their limits, this pushes %s over budget.",
	"track.apply":         "Track %d",
	"track.applied":       "Now tracking %s.",
	// The strip's bulk entry point.
	"budgets.trackAll": "Review all untracked",
	// %d = how many untracked categories exist this period, INCLUDING the ones the
	// four-chip cap does not show.
	"budgets.trackAllCount": "Review all %d untracked",
}

func init() {
	for k, v := range trackUntrackedKeys {
		english[k] = v
	}
}
