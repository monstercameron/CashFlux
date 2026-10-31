// SPDX-License-Identifier: MIT

package styles

func registerRulesUXT3() {
	rule(".txn-desc-account",
		display("none"),
	)
	ruleMedia("(max-width: 1180px)", ".txn-responsive-toolbar .filter-toolbar-primary",
		flexWrap("wrap"),
	)
	ruleMedia("(max-width: 1180px)", ".txn-responsive-toolbar .filter-toolbar-primary .fctrl-search",
		flex("1 1 0"),
		minWidth("0"),
	)
	ruleMedia("(max-width: 1180px)", ".txn-responsive-toolbar .filter-toolbar-primary .filters-trigger",
		whiteSpace("nowrap"),
	)
	ruleMedia("(max-width: 1180px)", ".txn-responsive-toolbar .filter-toolbar-actions",
		flexWrap("wrap"),
		minWidth("0"),
	)
	ruleMedia("(max-width: 1180px)", ".txn-responsive-toolbar .filter-toolbar-actions, .txn-responsive-toolbar .filter-toolbar-actions *",
		whiteSpace("nowrap"),
	)
	ruleMedia("(max-width: 1180px)", ".txn-responsive-toolbar .txn-toolbar-add, .txn-responsive-toolbar .txn-toolbar-add *",
		whiteSpace("nowrap"),
	)
	ruleMedia("(max-width: 1180px)", ".txn-responsive-toolbar .txn-toolbar-add",
		flex("0 0 auto"),
	)
	ruleMedia("(max-width: 1180px)", ".bento-ledger .txn-table",
		tableLayout("fixed"),
		width("100%"),
	)
	ruleMedia("(max-width: 1180px)", ".bento-ledger .txn-table tbody tr.row",
		display("table-row"),
	)
	ruleMedia("(max-width: 1180px)", ".bento-ledger .txn-table tbody tr.row > td",
		display("table-cell"),
		verticalAlign("middle"),
	)
	ruleMedia("(max-width: 1180px)", ".bento-ledger .txn-table thead",
		display("table-header-group !important"),
	)
	ruleMedia("(max-width: 1180px)", ".bento-ledger .txn-table thead th",
		display("table-cell !important"),
		width("auto"),
	)
	ruleMedia("(max-width: 1180px)", ".bento-ledger .txn-table .txn-col-who",
		display("none !important"),
	)
	ruleMedia("(max-width: 1180px)", ".bento-ledger .txn-table .txn-col-account",
		display("none !important"),
	)
	ruleMedia("(max-width: 1180px)", ".txn-desc-account",
		display("block"),
		flex("0 0 100%"),
		minWidth("0"),
		fontSize("var(--fs-meta)"),
		lineHeight("1.2"),
		color("var(--text-dim)"),
		overflow("hidden"),
		textOverflow("ellipsis"),
	)
	ruleMedia("(max-width: 1180px)", ".txn-table .row-desc-inner",
		flexWrap("wrap"),
	)
	ruleMedia("(max-width: 1180px)", ".txn-table .txn-desc-mainline",
		flexWrap("wrap"),
	)
	ruleMedia("(max-width: 1180px)", ".txn-table .txn-desc-mainline .row-desc-text",
		flex("1 1 120px"),
		minWidth("120px"),
	)
	ruleMedia("(max-width: 1180px)", ".txn-table .txn-col-amount",
		width("112px"),
		minWidth("112px"),
		textAlign("right"),
	)
	ruleMedia("(max-width: 1180px)", ".txn-table .txn-col-actions",
		width("76px"),
		minWidth("76px"),
		whiteSpace("nowrap"),
	)
	ruleMedia("(max-width: 1180px)", ".txn-table .td-date",
		width("94px"),
	)
	ruleMedia("(max-width: 1180px)", ".txn-table .td-select",
		width("38px"),
	)
	ruleMedia("(max-width: 1180px)", ".txn-table tbody tr.row",
		height("auto"),
		maxHeight("none"),
		minHeight("44px"),
	)
	ruleMedia("(max-width: 767px)", "main.cf-scroll:has(.txn-table)",
		paddingBottom("calc(112px + env(safe-area-inset-bottom, 0px)) !important"),
	)
	ruleMedia("(max-width: 767px)", ".txn-table tbody tr.row > td.td-actions .txn-row-menu, .txn-table tbody tr.row > td.td-actions .txn-row-menu button",
		maxWidth("100%"),
		boxSizing("border-box"),
	)
	ruleMedia("(max-width: 767px)", ".txn-table tbody tr.row > td.td-actions .txn-row-menu",
		minWidth("0"),
		overflow("visible"),
	)
}
