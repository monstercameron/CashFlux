// SPDX-License-Identifier: MIT

package styles

// registerLane3Mobile emits the #59 mobile design pass. The root defect: the
// phone bento override used `1fr` tracks (whose implicit minimum is
// min-content), so any tile with an unshrinkable row — the accounts net-worth
// strip, the budgets status strip, a goals loader — silently widened its track
// past the viewport and the whole page scrolled sideways. Tracks are clamped
// to minmax(0,1fr) and the widest in-tile rows stack or wrap at phone widths.
func registerLane3Mobile() {
	// ── Tracks can actually shrink ───────────────────────────────────────────
	ruleMedia("(max-width: 767px)", ".bento",
		gridTemplateColumns("minmax(0, 1fr) !important"),
	)
	ruleMedia("(max-width: 767px)", ".bento > *",
		minWidth("0"),
	)
	ruleMedia("(max-width: 767px)", ".bento .w",
		overflowX("hidden"),
	)
	// The tile's will-change:transform compositor hint makes every tile the
	// containing block for position:fixed descendants — which pins the row
	// menu's bottom sheet to the TILE's bottom, thousands of px off-screen.
	// Phones render single-column with no drag choreography; drop the hint.
	ruleMedia("(max-width: 640px)", ".bento .w",
		prop("will-change", "auto !important"),
	)

	// ── Accounts: the net-worth trio stacks two-up, stats wrap ───────────────
	ruleMedia("(max-width: 640px)", ".nw-summary",
		display("grid"),
		gridTemplateColumns("repeat(2, minmax(0, 1fr))"),
		gap("0.6rem"),
	)
	ruleMedia("(max-width: 640px)", ".nw-summary .stat",
		minWidth("0"),
	)
	ruleMedia("(max-width: 640px)", ".nw-summary .stat-value",
		fontSize("1.05rem"),
		overflowWrap("anywhere"),
	)

	// ── Budgets: the status strip stacks its cells ───────────────────────────
	ruleMedia("(max-width: 640px)", ".budgets-status-strip",
		display("flex"),
		flexDirection("column"),
		alignItems("stretch"),
		gap("0.6rem"),
	)
	ruleMedia("(max-width: 640px)", ".budgets-status-strip > *",
		minWidth("0"),
		width("100%"),
	)

	// ── Goals / To-do: headline loader figures wrap instead of overlapping ───
	ruleMedia("(max-width: 640px)", ".budget-loader-figs",
		flexWrap("wrap"),
		rowGap("0.35rem"),
		minWidth("0"),
	)
	ruleMedia("(max-width: 640px)", ".budget-loader-figs > *",
		minWidth("0"),
	)

	// ── Transactions: the filter toolbar wraps, search takes its own row ─────
	ruleMedia("(max-width: 640px)", ".filter-toolbar, .filter-toolbar-primary",
		flexWrap("wrap"),
		rowGap("0.5rem"),
		minWidth("0"),
	)
	ruleMedia("(max-width: 640px)", ".fctrl-search",
		flexBasis("100%"),
		minWidth("0"),
	)

	// ── Touch targets: labeled controls reach 44px on phones ─────────────────
	ruleMedia("(max-width: 640px)", "main .btn, main .fctrl select, main .fctrl input, main .fctrl-input",
		minHeight("44px"),
	)

	// ── Accounts rows go two-line: identity + figure, actions underneath ─────
	ruleMedia("(max-width: 640px)", ".acct-row-head",
		flexWrap("wrap"),
		rowGap("0.45rem"),
	)
	ruleMedia("(max-width: 640px)", ".acct-row-actions",
		flexBasis("100%"),
		flexWrap("wrap"),
		rowGap("0.4rem"),
		justifyContent("flex-start"),
		minWidth("0"),
	)

	// ── Budgets compact rows: 6-track desktop grid → stacked phone card ──────
	// Name + amount on the first line, the bar full-width beneath, then the
	// remaining cells (left, chips, kebab) auto-flowing two-up.
	ruleMedia("(max-width: 640px)", ".budget-crow",
		gridTemplateColumns("minmax(0, 1fr) max-content"),
		rowGap("0.35rem"),
	)
	ruleMedia("(max-width: 640px)", ".budget-crow-bar",
		gridColumn("1 / -1"),
	)

	// ── Filter/toolbar controls can shrink instead of forcing width ──────────
	ruleMedia("(max-width: 640px)", ".fctrl",
		minWidth("0"),
		maxWidth("100%"),
		flexWrap("wrap"),
	)
	ruleMedia("(max-width: 640px)", ".fctrl select",
		maxWidth("100%"),
	)
	ruleMedia("(max-width: 640px)", ".filter-toolbar-actions",
		flexWrap("wrap"),
		minWidth("0"),
	)

	// ── Goals cards: titles wrap, loader figures never overlap ───────────────
	ruleMedia("(max-width: 640px)", ".goal-card-head, .goal-card-title",
		minWidth("0"),
		overflowWrap("anywhere"),
	)
	ruleMedia("(max-width: 640px)", ".goal-card-loader-figs, .bento-goals .goal-card-loader-figs, .goal-figs",
		flexWrap("wrap"),
		rowGap("0.3rem"),
		minWidth("0"),
	)
	// The goals list track (1fr) still floors at min-content; clamp it and let
	// rows shrink so a 320px viewport can't be forced sideways.
	ruleMedia("(max-width: 640px)", ".bento-goals .goal-list",
		gridTemplateColumns("minmax(0, 1fr)"),
	)
	ruleMedia("(max-width: 640px)", ".goal-row, .goal-card-loader, .goal-card-head",
		minWidth("0"),
		maxWidth("100%"),
		overflowX("hidden"),
	)

	// ── 320px: the action row and period picker go compact ───────────────────
	// The smart-insights chip and the Add split-caret fold below 360px (the
	// insight strip stays reachable from the page; Add keeps its main button).
	ruleMedia("(max-width: 360px)", ".topbar .smart-peek, .topbar .add-caret",
		display("none !important"),
	)
	ruleMedia("(max-width: 360px)", ".topbar .tb-actions",
		gap("0.05rem"),
	)
	ruleMedia("(max-width: 640px)", ".period-control .btn, .period-control .period-step",
		paddingLeft("0.45rem"),
		paddingRight("0.45rem"),
	)
	// Ledger tag chips shrink with the row instead of shoving it sideways.
	ruleMedia("(max-width: 640px)", ".txn-desc-tag",
		flexShrink("1"),
		maxWidth("6rem"),
	)

	// ── Transactions rows: designed two-line cards, not one-cell-per-line ────
	// Line 1: checkbox · description (grows) · amount. Line 2: date + the
	// secondary columns as a quiet meta strip, with the ⋯ actions pinned right.
	ruleMedia("(max-width: 640px)", ".txn-table tbody tr.row",
		display("flex"),
		flexWrap("wrap"),
		alignItems("center"),
		prop("column-gap", "0.55rem"),
		rowGap("0.1rem"),
		padding("0.5rem 3.2rem 0.5rem 0.55rem"),
		position("relative"),
	)
	ruleMedia("(max-width: 640px)", ".txn-table tbody tr.row > td",
		display("block"),
		width("auto"),
		minWidth("0"),
		padding("0"),
	)
	ruleMedia("(max-width: 640px)", ".txn-table tbody tr.row > td.row-desc-cell",
		order("1"),
		// Grow to fill line one beside the amount, but never starve below 6.5rem
		// (a zero floor truncated every payee to an ellipsis; 8rem + the amount
		// cell overflowed the line by ~20px and pushed the amount down).
		flex("1 1 0"),
		minWidth("6.5rem"),
	)
	ruleMedia("(max-width: 640px)", ".txn-table tbody tr.row > td.td-amount",
		order("2"),
		textAlign("right"),
		fontSize("0.95rem"),
	)
	ruleMedia("(max-width: 640px)", ".txn-table tbody tr.row > td:nth-child(2), .txn-table tbody tr.row > td.td-acct, .txn-table tbody tr.row > td.td-cat",
		order("3"),
		fontSize("var(--type-11)"),
		color("var(--text-dim)"),
		flex("0 1 auto"),
		whiteSpace("nowrap"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		maxWidth("40vw"),
	)
	// Source and member are tertiary on a phone — the edit modal still carries
	// them; dropping them is what makes a true two-line row possible.
	ruleMedia("(max-width: 640px)", ".txn-table tbody tr.row > td.td-source, .txn-table tbody tr.row > td.td-user",
		display("none"),
	)
	// At true-narrow widths (320px) the select checkbox yields — bulk select
	// stays available from 360px up — so payee + amount still share line one.
	ruleMedia("(max-width: 360px)", ".txn-table tbody tr.row > td:first-child",
		display("none"),
	)
	ruleMedia("(max-width: 360px)", ".txn-table tbody tr.row > td.row-desc-cell",
		minWidth("5.5rem"),
	)
	ruleMedia("(max-width: 360px)", ".txn-table tbody tr.row > td.td-amount",
		fontSize("var(--type-14)"),
	)
	// The ⋯ keeps its 44px target by floating right-center of the card instead
	// of consuming a text line (the row padding clears its column). Centered
	// via inset+flex, NOT translateY — a transform here would make this cell
	// the containing block for the menu's fixed-position bottom sheet.
	ruleMedia("(max-width: 640px)", ".txn-table tbody tr.row > td.td-actions",
		position("absolute"),
		right("0.45rem"),
		top("0"),
		bottom("0"),
		display("flex"),
		alignItems("center"),
	)

	// ── Row actions become a bottom sheet on phones ──────────────────────────
	// AnchorPopover positions the ⋯ menu inline; the !important overrides win
	// so the same menu presents as a thumb-reachable sheet at phone widths.
	ruleMedia("(max-width: 640px)", ".txn-table .add-menu:not(.hidden-menu)",
		position("fixed !important"),
		left("0 !important"),
		right("0 !important"),
		bottom("0 !important"),
		top("auto !important"),
		width("auto !important"),
		maxWidth("none !important"),
		borderRadius("16px 16px 0 0"),
		maxHeight("60vh"),
		overflowY("auto"),
		zIndex("60"),
		padding("0.5rem 0 calc(env(safe-area-inset-bottom, 0px) + 0.5rem)"),
		boxShadow("0 -8px 24px rgba(0, 0, 0, 0.4)"),
		// AnchorPopover positions via inline transform/margins — neutralize so
		// the sheet actually pins to the bottom edge at full width.
		transform("none !important"),
		margin("0 !important"),
		minWidth("0 !important"),
	)
	ruleMedia("(max-width: 640px)", ".txn-table .add-menu:not(.hidden-menu) .add-item",
		minHeight("48px"),
		padding("0.6rem 1.1rem"),
	)
	ruleMedia("(max-width: 640px)", ".txn-table .add-backdrop:not(.hidden-menu)",
		position("fixed"),
		inset("0"),
		background("rgba(0, 0, 0, 0.45) !important"),
		zIndex("59"),
	)
}
