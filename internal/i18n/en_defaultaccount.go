// SPDX-License-Identifier: MIT

package i18n

var defaultAccountKeys = Catalog{
	"defaultAccount.pill":               "Default",
	"defaultAccount.pillTitleHousehold": "Default account for new household transactions.",
	"defaultAccount.pillTitleMembers":   "Default account for %s.",
	"defaultAccount.pillTitleBoth":      "Default for new household transactions and %s.",
	"defaultAccount.makeHousehold":      "Make default for new transactions",
	"defaultAccount.clearHousehold":     "Clear household default",
	"defaultAccount.makeMember":         "Make default for %s",
	"defaultAccount.clearMember":        "Clear %s’s default account",
	"defaultAccount.householdLabel":     "Default account for new transactions",
	"defaultAccount.householdHint":      "Choose the account to start with when no member-specific default applies.",
	"defaultAccount.none":               "No household default",
	"defaultAccount.householdSet":       "Household default account saved.",
	"defaultAccount.householdCleared":   "Household default account cleared.",
	"defaultAccount.memberSet":          "%s’s default account saved.",
	"defaultAccount.memberCleared":      "%s’s default account cleared.",
	"defaultAccount.memberMissing":      "That household member could not be found.",
	"defaultAccount.archivedError":      "Archived accounts cannot be a default account.",
}

func init() {
	for key, value := range defaultAccountKeys {
		english[key] = value
	}
}
