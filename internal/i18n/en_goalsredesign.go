// SPDX-License-Identifier: MIT

package i18n

// goalsRedesignKeys holds English strings for the /goals surface redesign
// (Territory B): the full-width goal-card figures grid, the optional (toggle-on)
// contribution planner disclosure, and the more-visual earmarks views. Kept in its
// own file (not en.go) so the redesign never touches the concurrent-WIP catalog.
// Registered at init time.
var goalsRedesignKeys = Catalog{
	// --- Full-width goal card: figures grid (scannable stat cells) ---
	"goalsredesign.figToGo":    "To go",
	"goalsredesign.figMonthly": "Monthly",
	"goalsredesign.figTarget":  "Target date",

	// --- Optional contribution planner (opt-in disclosure) ---
	// The chip that reveals / hides the "Plan your contribution" slider.
	"goalsredesign.planShow": "Plan contribution",
	"goalsredesign.planHide": "Hide planner",

	// --- Earmarks: account exposure coverage bar (accessible label). ---
	// %s = account name, %s = earmarked amount, %s = current balance.
	"goalsredesign.earmarkBarLabel": "%s: %s earmarked of %s balance",
	// Per-goal coverage bar (accessible label). %d = coverage percent.
	"goalsredesign.coverageBarLabel": "%d%% of the target is covered",

	// Goal notes (free-text, shown on the card; edited in the goal editor).
	"goals.notesLabel":       "Notes",
	"goals.notesPlaceholder": "Why this goal matters, the plan, reminders…",
}

func init() {
	for k, v := range goalsRedesignKeys {
		english[k] = v
	}
}
