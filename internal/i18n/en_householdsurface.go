// SPDX-License-Identifier: MIT

package i18n

// householdSurfaceKeys holds the English strings for the redesigned /household
// people-ledger surface: the hero (household net worth + figure chips + the
// plain-English takeaway), the roster rows, and the per-person analytics tab.
// Merged via init so this file does not touch en.go.
var householdSurfaceKeys = Catalog{
	"household.heroTitle":    "Your household",
	"household.heroLabel":    "Household net worth",
	"household.peopleOne":    "1 person",
	"household.peopleN":      "%d people",
	"household.baseSuffix":   "%s base",
	"household.chipSpend":    "Spending this period",
	"household.chipIncome":   "Income this period",
	"household.chipShared":   "Shared pot",
	"household.chipPeople":   "People",
	"hh.holderLead":          "%s holds the largest share of the household's worth.",
	"hh.holderAll":           "Everything the household owns sits in the shared pot.",
	"hh.spendClauseShared":   "All of this period's spending is shared.",
	"hh.spendClauseTop":      "%s did most of this period's spending.",
	"hh.spendClauseNone":     "No spending recorded this period yet.",
	"household.rosterTitle":  "Who's in the house",
	"members.worthLabel":     "net worth",
	"members.spentSub":       "spent %s this period",
	"members.shareOfWorth":   "share of household worth",
	"members.shareOfWorthNeg": "weighs on household worth (owes more than they hold)",
	"members.pinBadge":       "PIN set",
	"members.menuAria":       "Member actions",
	"household.byPersonTitle": "Where each person stands",
	"members.netWorthTake":   "Net worth by person, largest share first.",
	"members.spendTake":      "Who spent what this period.",
	"members.incomeTake":     "This period's income, split evenly across people.",
	"members.customLegend":   "Your fields",
}

func init() {
	for k, v := range householdSurfaceKeys {
		english[k] = v
	}
}
