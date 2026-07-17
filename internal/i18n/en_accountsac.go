// SPDX-License-Identifier: MIT

package i18n

// accountsACKeys holds the English strings for the /accounts groups + sparkline +
// flow features (AC1/AC2/AC9). Kept in their own file and merged via init so this
// does not touch the shared en.go.
var accountsACKeys = Catalog{
	// AC1 — account groups.
	"accounts.groupsAction": "Groups",
	// QA CF-27: the menu item says what clicking does, by state.
	"accounts.newGroupAction":     "New group…",
	"accounts.manageGroupsAction": "Manage groups",
	"accounts.groupsManageTitle":  "Organize your accounts into groups",
	"accounts.newGroup":           "New group",
	"accounts.editGroup":          "Edit group",
	"accounts.groupNameLabel":     "Group name",
	"accounts.groupNamePh":        "Shared, Liquid, Property…",
	"accounts.groupNameRequired":  "Give the group a name.",
	"accounts.groupPickAccounts":  "Choose which accounts belong to this group.",
	"accounts.groupNoAccounts":    "Add an account first, then group it.",
	"accounts.createGroup":        "Create group",
	"accounts.saveGroup":          "Save group",
	"accounts.deleteGroup":        "Delete group",
	// %s = group name. Deleting only ungroups; accounts are untouched.
	"accounts.groupDeleted":     "Removed the \"%s\" group. Its accounts are back under Ungrouped.",
	"accounts.groupSaved":       "Group saved.",
	"accounts.ungroupedSection": "Ungrouped",
	// %s = formatted net subtotal.
	"accounts.groupSubtotal":     "Net %s",
	"accounts.groupSubtotalAria": "Group net subtotal: %s",
	"accounts.editGroupTitle":    "Rename this group or change its accounts",
	"accounts.groupRowAria":      "Account group %s",
	// AC2 — balance sparkline.
	// %s = account name. Describes the 90-day trend line for screen readers.
	"accounts.sparklineAria": "%s balance over the last 90 days",
	"accounts.sparklineFlat": "%s balance has not moved in 90 days",
	// AC9 — in/out flow columns.
	"accounts.flowIn":  "In",
	"accounts.flowOut": "Out",
	"accounts.flowNet": "Net",
	// %s each = formatted money in / out / net for this period.
	"accounts.flowAria": "This period: %s in, %s out, %s net",
}

func init() {
	for k, v := range accountsACKeys {
		english[k] = v
	}
}
