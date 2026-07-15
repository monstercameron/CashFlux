// SPDX-License-Identifier: MIT

package i18n

// roundUpKeys holds English strings for the TX11 virtual round-ups feature: the
// sweep card on /goals, the config flip modal, and the goal-row jar. Kept out of
// en.go (concurrent-WIP file) and registered into the catalog at init time.
var roundUpKeys = Catalog{
	// Card.
	"roundups.cardTitle": "Round-ups",
	// %s = accrued amount, %s = cadence phrase ("this week"/"this month"), %s = goal name.
	"roundups.cardBody":     "You've rounded up %s %s. Add it to %s?",
	"roundups.addAction":    "Add %s to %s", // %s = amount, %s = goal name
	"roundups.dismiss":      "Not now",
	"roundups.done":         "Added %s to %s from round-ups.", // %s = amount, %s = goal
	"roundups.thisWeek":     "this week",
	"roundups.thisMonth":    "this month",
	"roundups.needGoal":     "Pick a goal for round-ups in the round-up settings first.",
	"roundups.fallbackGoal": "your goal",

	// Goal-row jar (running total on the target goal).
	"roundups.jar": "%s in round-ups", // %s = accrued amount

	// Config toolbar button + modal.
	"roundups.openConfig":        "Round-ups",
	"roundups.configTitle":       "Round-ups",
	"roundups.configIntro":       "Round every expense up to the next dollar and add the spare change to a goal — no real transfers, just a running jar you approve.",
	"roundups.configEnable":      "Turn round-ups on",
	"roundups.configGoal":        "Add round-ups to",
	"roundups.configGoalNone":    "Choose a goal…",
	"roundups.configNoGoals":     "Add a goal first, then choose it here.",
	"roundups.configCadence":     "Sweep the jar",
	"roundups.cadenceWeekly":     "Weekly",
	"roundups.cadenceMonthly":    "Monthly",
	"roundups.configAccounts":    "Accounts that participate",
	"roundups.configAllAccounts": "Leave all unchecked to include every account.",
	"roundups.configNoAccounts":  "Add an account first.",
	"roundups.cancel":            "Cancel",
	"roundups.save":              "Save",
}

func init() {
	for k, v := range roundUpKeys {
		english[k] = v
	}
}
