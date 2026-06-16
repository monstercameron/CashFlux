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
	"common.notReady": "App state is not ready yet.",

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
