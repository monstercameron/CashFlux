// SPDX-License-Identifier: MIT

package i18n

// accountsStmtKeys holds English strings for the account-metadata batch (Agent B):
// AC3 statement/due-day metadata, AC4 liability carrying cost, AC11 exclude-from-
// net-worth, and AC15 idle-cash. Kept in its own file (not en.go) so this change
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
	// AC4: household total carrying cost. %s = total monthly interest.
	"accountsstmt.carryingCostTotal": "Your debts cost about %s/month in interest to hold",
	// AC4: link to the payoff page.
	"accountsstmt.carryingCostLink": "See payoff plan",

	// AC15: idle-cash line. %s = idle amount, %s = forgone yearly yield, %s = benchmark rate.
	"accountsstmt.idleCash": "About %s is sitting idle — it could earn ~%s/year at your %s benchmark rate.",
	// AC15: when no benchmark rate is set yet.
	"accountsstmt.idleCashNoBenchmark": "About %s is sitting idle. Set a benchmark savings rate to see what it could earn.",
	// AC15: link to the allocate page.
	"accountsstmt.idleCashLink": "Put it to work",
}

func init() {
	for k, v := range accountsStmtKeys {
		english[k] = v
	}
}
