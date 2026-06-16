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
