// SPDX-License-Identifier: MIT

package i18n

// xcKeys holds English strings for the cross-concept features XC5 (price-creep
// accept flow) and XC9 (payday pre-flight checklist). Kept in a separate file
// (not en.go) so this change does not touch the concurrent-WIP catalog file.
var xcKeys = Catalog{
	// --- XC9 payday pre-flight ---
	"preflight.title":          "Payday check-in",
	"preflight.subtitle":       "Before your next paycheck on %s, here's what's coming.",
	"preflight.lowPoint":       "Your balance is projected to dip to %s around %s.",
	"preflight.lowPointFloor":  "That's below your %s keep-floor — plan a move.",
	"preflight.lowPointOk":     "That stays above your keep-floor.",
	"preflight.billsHeading":   "Bills due this cycle",
	"preflight.autopay":        "Autopay",
	"preflight.markPlanned":    "Mark planned",
	"preflight.planned":        "Planned",
	"preflight.transfer":       "Move money",
	"preflight.dippingHeading": "Accounts running thin",
	"preflight.dippingItem":    "%s holds %s — below your keep-floor.",
	"preflight.dismiss":        "Dismiss",
	"preflight.empty":          "Nothing needs attention this cycle.",

	// --- XC5 price-creep accept flow ---
	"pricecreep.acceptTitle":    "Accept the new price?",
	"pricecreep.crept":          "%s was set at %s but now charges about %s.",
	"pricecreep.impactBefore":   "%s budget: %d%% used now.",
	"pricecreep.impactAfter":    "After the increase: %d%% used.",
	"pricecreep.impactNoBudget": "This bill isn't in a budget yet.",
	"pricecreep.acceptPrice":    "Accept new price",
	"pricecreep.acceptAndRaise": "Accept and raise the budget",
	"pricecreep.makeTask":       "Make it a task to cancel",
	"pricecreep.cancel":         "Not now",
	"pricecreep.accepted":       "Updated %s to %s.",
	"pricecreep.taskCreated":    "Added a task to cancel or downgrade %s.",
	// Notice line on /recurring naming the drift, with the Review action beside it.
	"pricecreep.noticeLine": "%s has gone up: set at %s, now charging about %s.",
	"pricecreep.review":     "Review",
	// Collapsed summary when several bills crept at once (one line, not a wall).
	"pricecreep.summaryLine": "%d bills have gone up since their prices were set.",
	"pricecreep.showAll":     "Review them",
}

func init() {
	for k, v := range xcKeys {
		english[k] = v
	}
}
