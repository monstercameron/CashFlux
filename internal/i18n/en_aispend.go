// SPDX-License-Identifier: MIT

package i18n

// aispendKeys holds English copy for the AI spend meter (EC-15). Kept in its own
// file so it doesn't touch the concurrently-edited main catalog.
var aispendKeys = Catalog{
	"aispend.title": "What AI has cost you this month",
	// The empty state says "nothing yet" rather than "$0.00": a month with no
	// recorded calls is not a month that cost nothing to run, it is a month with
	// no data, and showing a total reads as a broken meter.
	"aispend.none": "Nothing yet this month.",
	// %s = a token count, %d = a number of calls.
	"aispend.thisMonth": "%s tokens across %d call(s)",
	"aispend.calls":     "%d call(s)",
	// %s = a cost. Used when some calls ran on a model with no known price, so
	// the figure is a floor rather than a total.
	"aispend.atLeast": "at least %s",

	"aispend.feature.assistant":  "Assistant",
	"aispend.feature.smart":      "Smart features",
	"aispend.feature.receipt":    "Receipt reading",
	"aispend.feature.categorize": "Categorizing",
	"aispend.feature.other":      "Other",

	// The pace lines. %s = the projected month total, %s = the cap.
	"aispend.paceTight":    "On track for about %s this month, against your %s limit.",
	"aispend.paceOver":     "At this rate you'll reach about %s this month — past your %s limit.",
	"aispend.paceExceeded": "You've passed your %s limit for this month.",

	"aispend.capNone":  "Set a monthly limit",
	"aispend.capSet":   "Monthly limit: %s",
	"aispend.capClear": "No limit",
	"aispend.capAria":  "Monthly AI spending limit in dollars",
}

func init() {
	for k, v := range aispendKeys {
		english[k] = v
	}
}
