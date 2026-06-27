// SPDX-License-Identifier: MIT

package i18n

// setupKeys holds the English strings for the guided /setup wizard (C21).
// Defined in their own file and merged via init so this does not touch the
// user-WIP en.go; mirrors the en_plans.go pattern.
var setupKeys = Catalog{
	"setup.pageTitle": "Set up CashFlux",
	"setup.pageSub":   "A quick, guided setup to get you started.",

	// Step 1 — currency & week start
	"setup.step1Label":      "Currency",
	"setup.step1Title":      "Your base currency",
	"setup.step1Hint":       "Everything is shown and totaled in this currency. You can change it later in Settings.",
	"setup.currencyLabel":   "Base currency",
	"setup.weekStartLabel":  "Week starts on",
	"setup.weekSunday":      "Sunday",
	"setup.weekMonday":      "Monday",
	"setup.weekSaturday":    "Saturday",
	"setup.confirmCurrency": "Continue",

	// Step 2 — income
	"setup.step2Label":       "Income",
	"setup.step2Title":       "Your monthly income",
	"setup.step2Hint":        "Used for budgeting and safe-to-spend. Leave blank to estimate it from your income transactions.",
	"setup.incomeLabel":      "Monthly take-home income",
	"setup.incomePlaceholder": "e.g. 4,000",
	"setup.confirmIncome":    "Continue",
	"setup.skipIncome":       "Skip for now",

	// Step 3 — first account
	"setup.step3Label":          "Account",
	"setup.step3Title":          "Add your first account",
	"setup.step3Hint":           "Add a checking, savings, or credit account to start tracking your money.",
	"setup.acctNameLabel":       "Account name",
	"setup.acctNamePlaceholder": "e.g. Everyday Checking",
	"setup.acctNameRequired":    "Please enter an account name.",
	"setup.acctTypeLabel":       "Type",
	"setup.acctBalLabel":        "Current balance",
	"setup.acctBalPlaceholder":  "e.g. 1,200",
	"setup.addAccount":          "Add account",
	"setup.acctAlreadyHave":     "You already have %d account(s).",
	"setup.skipAccount":         "Skip for now",

	// Step 4 — household members (optional)
	"setup.step4Label":           "Members",
	"setup.step4Title":           "Add household members",
	"setup.step4Hint":            "Optional. Add the people who share this household so you can track per-member views later.",
	"setup.memberNameLabel":      "Member name",
	"setup.memberNamePlaceholder": "e.g. Alex",
	"setup.memberNameRequired":   "Please enter a name.",
	"setup.addMember":            "Add member",
	"setup.membersAlready":       "Your household has %d member(s).",
	"setup.skipMembers":          "Skip — it's just me",

	// Completion
	"setup.doneTitle":       "You're all set!",
	"setup.doneBody":        "Your basics are configured. Jump in and start tracking your money.",
	"setup.doneBodyPartial": "You can finish the remaining steps any time from the setup checklist in Help.",
	"setup.goDashboard":     "Go to dashboard",
}

func init() {
	for k, v := range setupKeys {
		english[k] = v
	}
}
