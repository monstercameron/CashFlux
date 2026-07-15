// SPDX-License-Identifier: MIT

package i18n

// ageMoneyKeys holds the copy for the budgets-summary "Age of money" stat: the
// label, the plain-English explainer for a healthy vs. tight buffer, the whole
// span caption, and the not-ready state shown before there's enough history.
// Merged via init so this file does not touch en.go.
var ageMoneyKeys = Catalog{
	"budgets.ageMoneyLabel": "Age of money",
	// %d is the age in whole days. Shown as the stat's figure caption.
	"budgets.ageMoneyDays": "%d days",
	// Healthy (larger) buffer: %d = days.
	"budgets.ageMoneyHealthy": "You're spending money you earned about %d days ago — a healthy buffer.",
	// Tight (small) buffer: %d = days.
	"budgets.ageMoneyTight": "You're spending money you earned only %d days ago — money moves through fast.",
	// One-day edge case reads naturally.
	"budgets.ageMoneyOneDay": "You're spending money almost as fast as it comes in.",
	// Capped at the one-year ceiling — the buffer is so deep the exact age stops mattering.
	"budgets.ageMoneyCapped": "You're spending money you earned over a year ago — an exceptional buffer.",
	// The "why?" affordance label + its tooltip explaining how the number is figured.
	"budgets.ageMoneyWhyLabel": "Why?",
	"budgets.ageMoneyWhy":      "Each dollar you spend is matched to the oldest dollar you'd earned but not yet spent. The age is how long that dollar waited, averaged over your recent spending.",
	// Figure unit (singular / plural), rendered next to the day count.
	"budgets.ageMoneyUnit":    "days",
	"budgets.ageMoneyUnitOne": "day",
	// Not-ready state — too little matched history to trust a figure.
	"budgets.ageMoneyNotReady": "Add a bit more income and spending history to see this.",
}

func init() {
	for k, v := range ageMoneyKeys {
		english[k] = v
	}
}
