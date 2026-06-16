package i18n

// english is the source catalog: the canonical key → English string map. New
// user-facing strings are added here (and used via T) as screens are migrated
// onto the language store. Keys are dot-namespaced by area.
var english = Catalog{
	// App + chrome
	"app.name":           "CashFlux",
	"household.title":    "Your household",
	"household.settings": "Settings",

	// Top bar
	"topbar.menu":     "Collapse menu",
	"topbar.add":      "Add a transaction",
	"topbar.addLabel": "+ Add",

	// Primary navigation
	"nav.dashboard":    "Dashboard",
	"nav.accounts":     "Accounts",
	"nav.transactions": "Transactions",
	"nav.budgets":      "Budgets",
	"nav.goals":        "Goals",
	"nav.todo":         "To-do",
	"nav.members":      "Members",
	"nav.categories":   "Categories",
	"nav.settings":     "Settings",

	// Rail sections
	"rail.myPages": "My pages",
	"rail.newPage": "New page",
	"rail.system":  "System",

	// Common actions (shared across screens)
	"action.add":    "Add",
	"action.save":   "Save",
	"action.cancel": "Cancel",
	"action.delete": "Delete",
	"action.edit":   "Edit",
	"action.close":  "Close",

	// Shared
	"common.notReady":      "App state is not ready yet.",
	"common.name":          "Name",
	"common.reassignTitle": "Reassign before deleting",
	"common.moveAndDelete": "Move and delete",

	// Category kind (shared)
	"category.expense": "Expense",
	"category.income":  "Income",

	// Categories screen
	"categories.add":            "Add category",
	"categories.nameRequired":   "Enter a category name.",
	"categories.pickDifferent":  "Pick a different category to move these into.",
	"categories.noParentTop":    "— No parent (top level) —",
	"categories.noParent":       "— No parent —",
	"categories.chooseCategory": "— Choose category —",
	"categories.parentOptional": "Parent category (optional)",
	"categories.parent":         "Parent category",
	"categories.reassignDesc":   "%q is used by %d transaction(s) or budget(s). Move them to another category, then it will be deleted.",
	"categories.expenseTitle":   "Expense categories",
	"categories.incomeTitle":    "Income categories",
	"categories.expenseEmpty":   "No expense categories yet.",
	"categories.incomeEmpty":    "No income categories yet.",
	"categories.editTitle":      "Edit category",
	"categories.deleteTitle":    "Delete category",

	"common.owner": "Owner",

	// Budgets screen
	"budgets.add":              "Add budget",
	"budgets.needCategory":     "Add an expense category first, then create budgets.",
	"budgets.limitRequired":    "Enter a positive limit.",
	"budgets.limitPlaceholder": "Limit (%s)",
	"budgets.limitLabel":       "Limit",
	"budgets.period":           "Period",
	"budgets.empty":            "No budgets yet.",
	"budgets.spent":            "Spent",
	"budgets.budgeted":         "Budgeted",
	"budgets.left":             "Left",
	"budgets.prevMonth":        "Previous month",
	"budgets.nextMonth":        "Next month",
	"budgets.overNear":         "%d over budget · %d near the limit",
	"budgets.onTrack":          "On track",
	"budgets.nearLimit":        "Near limit",
	"budgets.overBudget":       "Over budget",
	"budgets.editTitle":        "Edit budget",
	"budgets.deleteTitle":      "Delete budget",
	"budgets.rowSub":           "%s · %s · %d%% · %s left",

	// Goals screen
	"goals.add":               "Add goal",
	"goals.targetPlaceholder": "Target (%s)",
	"goals.savedSoFar":        "Saved so far",
	"goals.totalTarget":       "Total target",
	"goals.overallProgress":   "Overall progress",
	"goals.linkedOptional":    "Linked account (optional)",
	"goals.linked":            "Linked account",
	"goals.owner":             "Owner",
	"goals.noLink":            "— No linked account —",
	"goals.targetRequired":    "Enter a positive target amount.",
	"goals.invalidDate":       "Enter a valid target date (YYYY-MM-DD).",
	"goals.empty":             "No goals yet.",
	"goals.targetLabel":       "Target",
	"goals.contribute":        "Contribute",
	"goals.contributeTitle":   "Add to this goal",
	"goals.contributePrompt":  "Contribute how much to %s?",
	"goals.editTitle":         "Edit goal",
	"goals.deleteTitle":       "Delete goal",
	"goals.progressFmt":       "%d%% · %s to go",
	"goals.complete":          "Complete 🎉",
	"goals.bySuffix":          " · by %s",
	"goals.saveSuffix":        " · save %s/mo",
	"goals.linkedSuffix":      " · linked to %s",

	// Priority (shared)
	"priority.high":   "High",
	"priority.medium": "Medium",
	"priority.low":    "Low",

	// Ownership (shared)
	"owner.group": "Group (shared)",

	// Members screen
	"members.add":                "Add member",
	"members.name":               "Name",
	"members.nameRequired":       "Enter a member name.",
	"members.listTitle":          "Household members",
	"members.empty":              "No members yet.",
	"members.netWorthTitle":      "Net worth by member",
	"members.roleMember":         "Member",
	"members.roleDefault":        "Default member",
	"members.defaultBadge":       "Default",
	"members.makeDefault":        "Make default",
	"members.makeDefaultTitle":   "Make default member",
	"members.viewTitle":          "View this member's transactions",
	"members.editTitle":          "Edit member",
	"members.deleteTitle":        "Delete member",
	"members.reassignTitle":      "Reassign before deleting",
	"members.reassignDesc":       "%q owns %d account(s), budget(s), or goal(s). Move them to another owner, then this member will be deleted.",
	"members.moveAndDelete":      "Move and delete",
	"members.pickDifferentOwner": "Pick a different owner to move these to.",

	// To-do screen
	"todo.addTitle":         "Add task",
	"todo.titlePlaceholder": "What needs doing?",
	"todo.notesPlaceholder": "Notes (optional)",
	"todo.invalidDue":       "Enter a valid due date (YYYY-MM-DD).",
	"todo.empty":            "No tasks yet.",
	"todo.allDone":          "All done 🎉",
	"todo.hideDone":         "Hide done",
	"todo.showAll":          "Show all",
	"todo.listTitle":        "Tasks",
	"todo.taskPlaceholder":  "Task",
	"todo.notesEdit":        "Notes",
	"todo.due":              "due",
	"todo.toggle":           "Toggle complete",
	"todo.editTitle":        "Edit task",
	"todo.deleteTitle":      "Delete task",
}

// DefaultBundle returns a fresh bundle seeded with the English source catalog.
// The UI layer holds one instance and merges imported languages into it.
func DefaultBundle() *Bundle {
	b := NewBundle(English)
	for key, msg := range english {
		b.Set(English, key, msg)
	}
	return b
}
