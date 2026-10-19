// SPDX-License-Identifier: MIT

package styles

// registerTV7F keeps the transaction filter sheet to one scroll area and
// aligns the desktop bulk controls on the same 36 px baseline.
func registerTV7F() {
	rule(".filter-inline-panel.filter-sheet",
		overflow("hidden"),
	)
	rule(".filter-inline-panel.filter-sheet .filter-inline-header, .filter-inline-panel.filter-sheet .filter-sheet-footer",
		flex("0 0 auto"),
	)
	rule(".filter-inline-panel.filter-sheet .filter-inline-body",
		flex("1 1 0"),
		minHeight("0"),
		overflowY("auto"),
	)
	rule(".filter-inline-panel.filter-sheet .filter-search-options, .filter-inline-panel.filter-sheet .filter-pills",
		maxHeight("none"),
		overflow("visible"),
	)
	rule(".txn-bulkbar-overlay .txn-bulk-primary > *, .txn-bulkbar-overlay > .btn-tool, .txn-bulkbar-overlay .bulk-picker-trigger, .txn-bulkbar-overlay .cf-multipicker-trigger",
		boxSizing("border-box"),
		height("36px"),
		minHeight("36px"),
	)
}
