// SPDX-License-Identifier: MIT

package i18n

// householdKeys holds English strings for the /household hub screen
// (FEATURE_MAP §5.3). Registered at init time using the init-merge pattern so
// en.go (which may carry concurrent WIP) is never touched.
var householdKeys = Catalog{
	// Segmented tab-bar aria label and tab labels.
	"household.tabAriaLabel": "Household navigation",
	"household.tabMembers":   "Members",
	"household.tabSplit":     "Split",
	"household.tabByPerson":  "By person",

	// Empty state shown on the By person tab when no members exist.
	"household.byPersonEmpty": "Add household members to see per-person analytics.",
}

func init() {
	for k, v := range householdKeys {
		english[k] = v
	}
}
