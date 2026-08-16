// SPDX-License-Identifier: MIT

package i18n

// txnScopeKeys holds the strings for the transactions ledger's scope model
// (C574/C575): the one-line statement of what the ledger is showing, the
// "viewing as" member-lens chip, and the clear/close controls that now name
// exactly what they remove. Merged via init so this file never touches en.go.
var txnScopeKeys = Catalog{
	// The scope line above the ledger. It states the size of the view, its
	// denominator, and every scope that produced it, so no visible count is left
	// for the user to reverse-engineer.
	// (The DATE half of the scope is stated by the ledger's own period bar
	// immediately above this line, so it is deliberately not repeated here.)
	"transactions.scopeAll":      "Showing all %s",
	"transactions.scopeNarrowed": "Showing %d of %s",
	"transactions.scopeNet":      "net %s",
	"transactions.scopeLens":     "viewing as %s",
	"transactions.scopeAria":     "What this ledger is showing",

	// The member lens, shown as its own chip rather than folded in with the page's
	// filters — it comes from the top bar and its ✕ clears the top bar, not a filter.
	"transactions.lensChip":       "Viewing as %s",
	"transactions.lensChipRemove": "Show everyone again",

	// Chips that used to render their raw stored value: the "no category yet"
	// quick filter showed the literal "1", and a custom-field filter showed its
	// value with no hint of which field it belonged to.
	"transactions.chipUncategorized": "No category yet",
	"transactions.chipCustom":        "%s: %s",

	// One reset, and it counts what it will remove. The toolbar's second "Clear
	// filters" button did exactly the same thing as this and is gone.
	"transactions.clearAllFiltersN": "Clear all %s",

	// The review backlog is measured across the whole household; the ledger beside
	// it usually is not. The button says so on hover and to assistive tech.
	"transactions.reviewScopeTitle": "%d charges need review across the whole household — this count ignores the filters on this page",

	// Classification provenance (C579). The mark is one small word beside a
	// category nobody has confirmed; the sentences behind it say what filed the
	// row, and name the rule when one accounts for it.
	"transactions.autoMark":           "auto",
	"transactions.autoWhyRule":        "Filed automatically by your rule matching \"%s\". No one has confirmed it — open the transaction to change or confirm the category.",
	"transactions.autoWhyRuleUnnamed": "Filed automatically by one of your rules. No one has confirmed it — open the transaction to change or confirm the category.",
	"transactions.autoWhySource":      "Filed automatically when this came in from %s. No rule of yours accounts for this category, and no one has confirmed it.",
	"transactions.autoWhyUnknown":     "Filed automatically. No rule of yours accounts for this category, and no one has confirmed it.",
	"transactions.confirmCategory":    "Confirm this category",
	"transactions.confirmedCategory":  "Category confirmed — this row no longer reads as automatic.",

	// Shared toolbar controls whose labels were generic enough to be ambiguous
	// once several clear/close controls sat next to each other.
	"action.clearSearch":  "Clear search",
	"filters.closePanel":  "Close the filter panel — your filters stay applied",
	"filters.summaryLead": "Filtering by",
}

func init() {
	for k, v := range txnScopeKeys {
		english[k] = v
	}
}
