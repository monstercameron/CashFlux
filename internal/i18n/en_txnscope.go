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
	// Said only when a row could not be folded into the base currency (no exchange
	// rate saved for it). Without this the net silently covered fewer rows than the
	// count printed beside it.
	"transactions.scopeUnconverted": "%d not counted — no exchange rate",

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

	// The one primary entry point and its type-scoped menu (C573). Each label names
	// the KIND of thing being created, and the form it opens is already set that way.
	"transactions.addKindLabel": "Choose what kind of transaction to add",
	"transactions.addExpense":   "Expense — money going out",
	"transactions.addIncome":    "Income — money coming in",
	"transactions.addTransfer":  "Transfer between your accounts",
	// The section heading sits over "Import" and had to stop reading as a fourth
	// KIND of transaction: the three items above it are kinds, so a heading of the
	// same weight in the same list gets read as one. It names the different thing
	// this section is — a bulk route in, rather than one entry.
	"transactions.addFromFile": "Many at once",
	// The global "+" menu names the same three kinds the page's Add menu does, so a
	// user meets one vocabulary wherever they start (C573).
	"addmenu.expense": "Expense — money going out",
	"addmenu.income":  "Income — money coming in",
	// C604: headings that sort eleven creation actions by JOB. Money that moved
	// leads and needs no heading; these two mark where the everyday stops.
	"addmenu.sectionPlan":  "Plan & track",
	"addmenu.sectionSetup": "Set up",

	// The one-hop breadcrumb out of a cross-page correction (C581).
	"returnTo.back":          "Back to %s",
	"returnTo.aria":          "Return to %s",
	"returnTo.note":          "your filters and page are still there",
	"returnTo.dismiss":       "Dismiss this shortcut back",
	"returnTo.labelSearch":   "Transactions, filtered to \"%s\"",
	"returnTo.labelFiltered": "Transactions, with %s applied",
	"returnTo.labelPlain":    "Transactions",

	// Calendar month/year context (C603). Under "All dates" the caption named the
	// ledger's scope and nothing named the month the GRID was drawing, so a wall of
	// day numbers had no month anywhere on screen; and a leading/trailing square was
	// dimmed but otherwise announced as a bare number.
	"transactions.calShowingMonth":  "%s · showing %s",
	"transactions.calDayOtherMonth": "%s — %s, not part of the month shown",
	"transactions.calPicked":        "Showing %s only. Use the date bar above to widen it again.",
	"transactions.calPickedFromAll": "Showing %s only. \"All dates\" in the bar above brings the rest back.",

	// The Status column (C578): the row's state as a word, in a lane of its own.
	"transactions.colStatus":      "Status",
	"transactions.statusReviewed": "Reviewed",
	// C601: a charge can be settled AND unreviewed. The column shows the state that
	// asks for a person; this joins the other one back on for the tooltip.
	"transactions.statusAlso":    "%s · %s",
	"transactions.colStatusHint": "Spells out reconciled / cleared / needs review, so the row's state reads without decoding the ✓ ✓✓ • markers.",
	// C618: "Needs review" claims the row is waiting in the Review inbox. A charge
	// that already HAS a category (filed by a rule, or added without ticking
	// "Mark as reviewed") is not in that queue, so saying it needs review sent
	// people looking for a card that was never there. It is unconfirmed, which is
	// a different thing, and the tooltip says where it actually stands.
	"transactions.statusUnconfirmed":      "Not confirmed",
	"transactions.statusUnconfirmedTitle": "Already categorized, so it isn't waiting in the Review inbox — open the row to confirm or change its category.",

	// The pending band above the ledger (C582), collapsed to one fact by default.
	"transactions.upcomingSummary":     "%d scheduled this month, %s still to come",
	"transactions.upcomingShow":        "Show them",
	"transactions.upcomingHide":        "Hide",
	"transactions.upcomingShowAria":    "Show the scheduled items — %s",
	"transactions.upcomingHideAria":    "Hide the scheduled items — %s",
	"transactions.upcomingSeeSchedule": "See the whole schedule",

	// Shared toolbar controls whose labels were generic enough to be ambiguous
	// once several clear/close controls sat next to each other.
	"action.clearSearch": "Clear search",
	"filters.closePanel": "Close the filter panel — your filters stay applied",
	// C619: search is debounced, so for a moment the rows below still answer the
	// PREVIOUS query. Say so, rather than letting someone act on results their
	// typing has already excluded.
	"filters.searchPending": "Searching…",
	"filters.summaryLead":   "Filtering by",
}

func init() {
	for k, v := range txnScopeKeys {
		english[k] = v
	}
}
