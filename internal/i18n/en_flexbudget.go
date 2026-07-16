// SPDX-License-Identifier: MIT

package i18n

// flexBudgetKeys holds the English strings for flex budgeting (BG2): the fourth
// methodology where day-to-day spending is managed as ONE flex number, fixed
// commitments render as checkoffs, and non-monthly costs show a smoothed accrual.
// Kept in its own file and merged via init so it never touches the shared en.go.
var flexBudgetKeys = Catalog{
	// The method-picker label for the flex methodology.
	"settings.budgetMethodFlex": "Flex (one number for day-to-day)",

	// Flex meter (the single signature number).
	"flex.title":       "Flex spending",
	"flex.spentOf":     "%s of %s spent",
	"flex.spentSoFar":  "spent so far this month",
	"flex.left":        "%s left to spend",
	"flex.over":        "%s over your flex budget",
	"flex.noTarget":    "Set a flex number to start tracking day-to-day spending.",
	"flex.targetLabel": "Flex budget",
	"flex.editTarget":  "Set flex number",
	"flex.saveTarget":  "Save",

	// Redesigned flex hero: eyebrow, the split number + word, the context line, and
	// the pace read that turns the meter into an instrument.
	"flex.eyebrow":     "Flex spending",
	"flex.thisMonth":   "this month",
	"flex.leftWord":    "left to spend",
	"flex.overWord":    "over your flex budget",
	"flex.spentTarget": "%s spent of %s",
	"flex.daysLeft":    "%d days left",
	"flex.dayLeft":     "1 day left",
	"flex.paceOnTrack": "On track",
	"flex.paceTight":   "Getting tight",
	"flex.paceFast":    "Spending fast",
	"flex.paceOver":    "Over budget",
	"flex.emptyTitle":  "One number for day-to-day spending",
	"flex.emptyBody":   "Set a flex budget and every everyday purchase counts against it — no envelopes to juggle. Fixed bills and irregular costs stay tracked separately below.",
	"flex.setNumber":   "Set your flex number",
	"flex.editShort":   "Edit",

	// Classify action + assignment sheet.
	"flex.classify":       "Classify categories",
	"flex.classifyTitle":  "Sort each category into flex, fixed, or non-monthly",
	"flex.sheetTitle":     "Classify categories",
	"flex.sheetIntro":     "Group each spending category so the flex view knows how to treat it. Flex is pooled day-to-day spending; fixed is an expected bill; non-monthly is an irregular cost you set aside for.",
	"flex.sheetEmpty":     "Add some expense categories first, then come back to classify them.",
	"flex.sheetSave":      "Save classifications",
	"flex.sheetCancel":    "Cancel",
	"flex.classFlexShort": "Flex",
	"flex.classFixed":     "Fixed",
	"flex.classNonMonth":  "Non-monthly",

	// Fixed-commitment checklist.
	"flex.fixedHeading": "Fixed commitments",
	"flex.fixedEmpty":   "No fixed commitments yet — classify a bill category as fixed.",
	"flex.fixedCount":   "%d of %d paid",
	"flex.expected":     "Expected %s",
	"flex.paid":         "Paid",
	"flex.unpaid":       "Not yet paid",
	"flex.actualOf":     "%s of %s",

	// Non-monthly set-asides.
	"flex.nonMonthHeading": "Non-monthly set-asides",
	"flex.nonMonthEmpty":   "No non-monthly costs yet — classify an irregular category as non-monthly.",
	"flex.nonMonthCount":   "%s / mo set aside",
	"flex.setAside":        "%s / mo",
	"flex.spentThisPeriod": "%s spent",
}

func init() {
	for k, v := range flexBudgetKeys {
		english[k] = v
	}
}
