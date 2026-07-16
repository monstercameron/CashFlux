// SPDX-License-Identifier: MIT

package i18n

// txcFieldKeys holds strings for the transaction-level comp-parity features:
// per-transaction memo (TXC-2), exclude-from-reports (TXC-1), and the quick-filter
// presets (TXC-3). Merged via init so this file never touches en.go.
var txcFieldKeys = Catalog{
	// TXC-2 memo.
	"transactions.noteLabel":       "Note",
	"transactions.notePlaceholder": "Add a memo (e.g. split with Priya, she owes half)…",
	"transactions.hasNote":         "Has a note",

	// TXC-1 exclude from budgets & reports.
	"transactions.excludeLabel":     "Exclude from budgets & reports",
	"transactions.excludeHint":      "Still counts toward account balances — just not budgets, spend, or reports.",
	"transactions.excludedBadge":    "Excluded",
	"transactions.kebabExclude":     "Exclude from reports",
	"transactions.kebabInclude":     "Include in reports",
	"transactions.bulkExcludeShort": "Exclude",
	"transactions.bulkIncludeShort": "Include",
	"transactions.bulkOpExcluded":   "Excluded %d from reports",
	"transactions.bulkOpIncluded":   "Included %d in reports",

	// TXC-4 non-lossy merge preview.
	"duplicates.mergeCarry":    "Merge also keeps: %s",
	"duplicates.carryReceipts": "%s",
	"duplicates.carryCategory": "a category",
	"duplicates.carryNote":     "a note",
	"duplicates.carryPayee":    "a payee",
	"duplicates.carryLink":     "a linked bill/subscription",

	// Transaction → follow-up task (kebab): create a to-do linked to this charge. The
	// title is just the merchant/description (no "Follow up:" prefix) — the transaction
	// link already says it's a follow-up, and the prefix wastes the popover's limited room.
	"transactions.followUpTask":      "Add follow-up task…",
	"transactions.followUpTaskTitle": "%s",
	// Per-row follow-up indicator + quick link to the filtered to-dos.
	// %d open, %d total.
	"transactions.followUpsTitle":   "%d open of %d follow-up task(s) — view in To-do",
	"transactions.followUpsAria":    "%d open follow-up tasks, view in To-do",
	"transactions.followUpsPopHead": "%d open · %d total",
	"transactions.followUpsPopLink": "Open in To-do →",
	"transactions.followUpMarkDone": "Mark done",
	"transactions.followUpMarkOpen": "Mark not done",
	"transactions.followUpsMore":    "+%d more open — see all in To-do",
	"transactions.followUpsAllDone": "All follow-ups done ✓",
	// New "Transactions" option in the To-do link filter.
	"todo.linkTransactionPl": "Linked to transactions",

	// TXC-3 quick-filter presets.
	"transactions.presetsLabel":        "Quick filters",
	"transactions.presetUncategorized": "Uncategorized",
	"transactions.presetNeedsReview":   "Needs review",
	"transactions.presetLarge":         "Large ($100+)",
	"transactions.presetThisMonth":     "This month",
}

func init() {
	for k, v := range txcFieldKeys {
		english[k] = v
	}
}
