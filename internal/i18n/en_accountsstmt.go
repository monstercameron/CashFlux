// SPDX-License-Identifier: MIT

package i18n

// accountsStmtKeys holds English strings for the account-metadata batch (Agent B):
// AC3 statement/due-day metadata, AC4 liability carrying cost, and AC11 exclude-from-
// net-worth. Kept in its own file (not en.go) so this change
// stays clear of the concurrent-WIP catalog. Registered into the catalog at init.
var accountsStmtKeys = Catalog{
	// AC3: statement-close day field on the liability edit form.
	"accountsstmt.statementDay": "Statement closes on day",

	// AC11: exclude-from-net-worth toggle on the account edit form.
	"accountsstmt.excludeNetWorth":     "Leave out of net worth",
	"accountsstmt.excludeNetWorthHint": "Still shown in its account list, just not counted in your net worth.",
	// AC11: net-worth disclosure line. %d = number of accounts excluded by choice.
	"accountsstmt.excludesByChoice": "Excludes %d account you chose to leave out",
	// Plural form. %d = count.
	"accountsstmt.excludesByChoicePlural": "Excludes %d accounts you chose to leave out",

	// AC4: per-row carrying-cost line. %s = monthly interest cost (e.g. "$43").
	"accountsstmt.carryingCost": "Costs about %s/month to hold",
}

func init() {
	for k, v := range accountsStmtKeys {
		english[k] = v
	}
}
