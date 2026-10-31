// SPDX-License-Identifier: MIT

package i18n

// accountsViewFixKeys holds English strings for the accounts page views: the notice for a failed
// account write, and the counted strings that come as a singular/plural pair. Counted strings are
// read with uistate.TN, which passes the count as the first argument, so no caller pluralises a
// noun itself.
var accountsViewFixKeys = Catalog{
	// A write the accounts page could not complete. The detail is logged, not shown.
	"accounts.writeFailed":   "That change could not be saved. Try again. If it keeps happening, reload the page.",
	"accounts.writeReadOnly": "This household is open in a viewing role, so accounts cannot be changed here.",
	"accounts.writeNotFound": "That account no longer exists. Reload the page and try again.",

	// A write rejected because of what the person entered. One sentence per field that can fail.
	"accounts.issue.name":            "Give the account a name.",
	"accounts.issue.owner":           "Choose who owns the account.",
	"accounts.issue.openingCurrency": "The opening balance must be in the account's currency.",
	"accounts.issue.score":           "Liquidity and stability must be between 0 and 100.",
	"accounts.issue.dueDay":          "The payment due day must be between 1 and 28.",
	"accounts.issue.statementDay":    "The statement day must be between 1 and 31.",
	"accounts.issue.apr":             "The interest rate cannot be negative.",
	"accounts.issue.generic":         "Some account details are not valid. Check the form and try again.",

	// More reasons a write can fail.
	"accounts.writeRateUnavailable": "There is no exchange rate for these currencies. Enter the amount received.",
	"accounts.writeStale":           "Your data changed in another tab. Reload and try again.",
	"accounts.writeNotReady":        "Saving is not ready yet, so nothing was changed. Keep this page open and try again.",

	// Names for buttons that repeat down a list. %s = the thing the button acts on.
	"accounts.editInstitutionAria": "Edit institution %s",
	"accounts.editGroupAria":       "Edit group %s",
	"accounts.documentOpenAria":    "Open %s",
	"accounts.documentRemoveAria":  "Remove %s",
	"sm6.makeRecurAria":            "Track %s as recurring",
	"sm6.trackRateUnavailable":     "There is no exchange rate for %s, so this repeating charge could not be tracked. Add the rate in Settings and try again.",

	// An institution that was removed while its editor was open.
	"accounts.institutionGone": "That institution no longer exists. Close this and reopen the directory.",

	// A sweep whose money moved but whose suggestion could not be marked as handled.
	"acctSweep.postedUnmarked": "The money was moved, but this suggestion could not be marked as done. Check your transfers before approving it again.",

	// A sweep approved for a different amount than the one this rule already moved in this period.
	"acctSweep.alreadyPosted": "This rule already moved money in this period, so nothing new was moved.",

	// Sweep rule removal. %s = the rule, written out as a sentence.
	"acctSweepCfg.removeAria":    "Remove this rule: %s",
	"acctSweepCfg.removeConfirm": "Remove this rule? %s",

	// Account group header. %d = number of accounts in the group.
	"accounts.groupCountOne":  "%d account",
	"accounts.groupCountMany": "%d accounts",

	// Institution directory row. %d = number of accounts at the institution.
	"accounts.institutionLinkedOne":  "%d account linked",
	"accounts.institutionLinkedMany": "%d accounts linked",

	// Bill payments linked to an account. %d = number of payments.
	"accounts.billCountOne":  "· %d payment →",
	"accounts.billCountMany": "· %d payments →",

	// Days since a balance was last updated, appended to the "Out of date" badge. %d = number of days.
	"accounts.staleDaysOne":  " · %d day",
	"accounts.staleDaysMany": " · %d days",

	// The projection never drops below today's balance, so it is flat or rising. %s = the closing balance.
	"accountsFwd.risingOrFlat": "Stays at or above today's balance, ending around %s",

	// Repeating charges found on one account. %d = how many were found.
	"sm6.hintOne":  "%d charge here repeats on a schedule.",
	"sm6.hintMany": "%d charges here repeat on a schedule.",
	// Shown when more repeats were found than the list holds. %d = how many are listed.
	"sm6.capNote": "Showing the first %d. The rest are on the Bills & recurring page.",

	// What the next-90-days line assumes. %d = number of scheduled bills and paychecks.
	"accountsFwd.assumesOne":  "Assumes your %d scheduled bill or paycheck keeps running as it is.",
	"accountsFwd.assumesMany": "Assumes your %d scheduled bills and paychecks keep running as they are.",
}

func init() {
	for k, v := range accountsViewFixKeys {
		english[k] = v
	}
}
