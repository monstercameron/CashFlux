// SPDX-License-Identifier: MIT

package i18n

// goalVisionKeys holds English strings for GL6 (goal vision images) and GL7
// (goal pause/snooze). Kept in a separate file (not en.go) so this change doesn't
// touch the concurrent-WIP catalog. Registered into the catalog at init time.
var goalVisionKeys = Catalog{
	// GL6 — vision images. Attach affordance + card banner.
	"goals.addPhoto":     "Add photo",
	"goals.changePhoto":  "Change photo",
	"goals.removePhoto":  "Remove photo",
	"goals.photoHint":    "A picture of what you're saving for.",
	"goals.photoAlt":     "Photo for %s",
	"goals.photoMissing": "Image unavailable",
	"goals.photoAdded":   "Photo added",
	"goals.photoRemoved": "Photo removed",

	// GL7 — pause / snooze. Menu actions, the pause form, and card chips.
	"goals.pauseAction":   "Pause goal",
	"goals.resumeAction":  "Resume goal",
	"goals.pauseTitle":    "Pause goal",
	"goals.pauseIntro":    "Take a break from this goal. Contributions won't be expected and the pace won't nudge you while it's paused — pause as long as you like.",
	"goals.pauseForLabel": "Pause for",
	"goals.pauseOneMonth": "1 month",
	// %d = number of months (2+).
	"goals.pauseMonths": "%d months",
	// Cost preview. %d = months, %s = original finish date, %s = shifted finish date.
	"goals.pauseCost": "Pausing %d months moves the finish from %s to %s.",
	// Single-month variant. %s = original finish, %s = shifted finish.
	"goals.pauseCostOne": "Pausing 1 month moves the finish from %s to %s.",
	// Shown when the goal has no dated finish to shift.
	"goals.pauseCostNoFinish": "This goal has no dated finish yet, so pausing just picks it up later.",
	"goals.pauseConfirm":      "Pause goal",
	// Toast after pausing. %s = pause-end date.
	"goals.pausedToast":  "Paused until %s",
	"goals.resumedToast": "Goal resumed",
	// Quiet card chip. %s = pause-end date.
	"goals.pausedChip": "Paused until %s",
	// Pause-end nudge task title. %s = goal name.
	"goals.pauseEndedNudge": "Your pause on %s has ended — pick it back up when you're ready.",
}

func init() {
	for k, v := range goalVisionKeys {
		english[k] = v
	}
}
