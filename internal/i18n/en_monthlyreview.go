// SPDX-License-Identifier: MIT

package i18n

// monthlyReviewKeys holds English copy for the guided month-end review (AG10).
// Kept in its own file so it doesn't touch the concurrently-edited main catalog.
var monthlyReviewKeys = Catalog{
	"review.title": "Month in review",
	// %d = the current step, %d = how many there are. Stated up front because the
	// first thing anybody wants to know about a guided flow is how long it is.
	"review.eyebrow": "Step %d of %d",
	"review.next":    "Next",
	"review.finish":  "Done",
	"review.skip":    "Skip this",
	"review.dismiss": "Not this month",

	"review.recapTitle":  "How the month went",
	"review.recapBody":   "Here's what you spent, against what's usual for you. Nothing to do yet — just get your bearings.",
	"review.recapAction": "See the figures",

	"review.findingsTitle":  "Things worth a look",
	"review.findingsAction": "Go through them",
	// %d = how many findings are waiting.
	"review.findingsBody": "%d thing(s) got flagged this month and haven't been dealt with. Some will be nothing.",

	"review.budgetsTitle":  "Budgets to true up",
	"review.budgetsAction": "Adjust them",
	// %d = how many budgets are over or close.
	"review.budgetsBody": "%d budget(s) are over or close to it. Either the budget is wrong or the month was — worth deciding which.",

	"review.goalsTitle":  "Goals that need a decision",
	"review.goalsAction": "Look at them",
	// %d = how many goals the coach has something to say about.
	"review.goalsBody": "%d goal(s) need a decision — behind, unfunded, or already there.",

	"review.closeTitle": "That's the month",
	// Shown under the close summary. It exists so ending the review does not feel
	// like it needed permission.
	"review.closeFine": "Nothing here is on a timer. Whatever you left is still there tomorrow.",
}

func init() {
	for k, v := range monthlyReviewKeys {
		english[k] = v
	}
}
