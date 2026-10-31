// SPDX-License-Identifier: MIT

package styles

// registerAccountsSurface emits the accounts-page rules: the invalid-field outline and the
// full-width add-form rows (.field.field-invalid, .acct-add-wide), the quick-pick row
// (.cf-dialog-choices), the liability toggle (.acct-liab-toggle), the account editor's value
// section (.acct-value-*), the account row, its notes line and its details toggle (.acct-row*,
// .acct-notes*, .acct-details-*), the account groups (.acct-group*), the balance sparkline
// (.acct-spark) and the sweep-rules add form (.sweep-add). It also chains the reconcile-flow and
// stale-account rules. Called from Register in install.go.
func registerAccountsSurface() {
	// A name field with a validation message is outlined in the danger colour, so the error reads
	// before the message is read. ClassIf and AttrIf set the marker from the form's error state.
	rule(".field.field-invalid",
		borderColor("var(--danger, #e5484d)"),
	)
	// The add-account form's full-width rows (the name, the copy starter, the "Other" liability
	// toggle and the advanced section) span both columns of the form grid.
	rule(".acct-add-wide",
		gridColumn("1 / -1"),
	)
	// The quick picks under a prompt's entry (the institutions already in the household) wrap onto
	// as many lines as they need.
	rule(".cf-dialog-choices",
		display("flex"),
		flexWrap("wrap"),
		gap(".4rem"),
	)
	// The liability toggle is a label that opens a sub-form, so it reads as clickable.
	rule(".acct-liab-toggle",
		cursor("pointer"),
	)
	// --- account editor: the value group at the top of the form ---
	// A subtly framed panel so the marquee account action (record a new value) reads as
	// its own section, distinct from the account-detail fields below it.
	rule(".acct-value-section",
		display("flex"),
		flexDirection("column"),
		gap("0.4rem"),
		padding("0.8rem 0.9rem"),
		marginBottom("0.9rem"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-lg)"),
		background("var(--hover)"),
	)
	// The "Currently $X" context sits just under the input; the delta preview under that.
	rule(".acct-value-now",
		fontSize("var(--type-13)"),
	)
	rule(".acct-value-delta",
		margin("0.05rem 0 0"),
	)

	// --- account row: readable, expandable notes line ---
	// The attached note itself, shown as a subtle framed disclosure that clamps to two
	// lines and expands on click — legible at a glance, not hidden in a hover tooltip.
	rule(".acct-notes",
		display("flex"),
		alignItems("flex-start"),
		gap("0.4rem"),
		boxSizing("border-box"),
		width("calc(100% - var(--acct-indent, 0rem))"),
		maxWidth("42rem"),
		textAlign("left"),
		marginTop("0.4rem"),
		padding("0.4rem 0.55rem"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-lg)"),
		background("transparent"),
		color("var(--text-dim)"),
		fontSize("var(--type-13)"),
		lineHeight("1.4"),
		cursor("pointer"),
		transition("background .15s ease, border-color .15s ease, color .15s ease"),
	)
	rule(".acct-notes:hover",
		background("var(--hover)"),
		borderColor("var(--text-dim)"),
		color("var(--text)"),
	)
	rule(".acct-notes:focus-visible",
		outline("2px solid var(--accent)"),
		outlineOffset("2px"),
	)
	rule(".acct-notes-icon",
		marginTop("0.12rem"),
		opacity("0.75"),
	)
	// Collapsed: clamp to two lines with an ellipsis. Preserves the note's own line
	// breaks (white-space:pre-wrap) so a multi-line note still reads naturally.
	rule(".acct-notes-text",
		display("-webkit-box"),
		prop("-webkit-line-clamp", "2"),
		prop("-webkit-box-orient", "vertical"),
		overflow("hidden"),
		whiteSpace("pre-wrap"),
		prop("word-break", "break-word"),
	)
	// Expanded: reveal the whole note.
	rule(".acct-notes.open .acct-notes-text",
		prop("-webkit-line-clamp", "unset"),
		overflow("visible"),
	)

	// --- account groups: sections with a subtotal header ---
	rule(".acct-group",
		marginBottom("0.6rem"),
	)
	// A quiet section header above each group's rows: name on the left, net subtotal on
	// the right, with a subtle rule underneath so the grouping reads without shouting.
	rule(".acct-group-header",
		padding("0.35rem 0.15rem 0.3rem"),
		marginTop("0.2rem"),
		borderBottom("1px solid var(--border)"),
		fontSize("var(--type-14)"),
	)
	rule(".acct-group-name",
		color("var(--text)"),
	)
	rule(".acct-group-subtotal",
		fontVariantNumeric("tabular-nums"),
	)
	// The sections stack in one column. Each header's toggle (chevron, name and count) is a
	// bare button, so it takes the header's type size and keeps the keyboard focus ring.
	rule(".acct-groups",
		display("flex"),
		flexDirection("column"),
		minWidth("0"),
	)
	rule(".acct-group-toggle",
		font("inherit"),
		minWidth("0"),
		background("transparent"),
		borderStyle("none"),
		padding("0"),
		margin("0"),
		cursor("pointer"),
		color("var(--text)"),
		textAlign("left"),
	)
	rule(".acct-group-toggle:focus-visible",
		outline("2px solid var(--accent)"),
		outlineOffset("2px"),
		borderRadius("var(--radius-sm, 4px)"),
	)
	rule(".acct-group-chevron",
		flexShrink("0"),
	)
	rule(".acct-group-count",
		fontSize("var(--type-13)"),
		fontVariantNumeric("tabular-nums"),
		whiteSpace("nowrap"),
	)

	// --- balance sparkline ---
	// A compact trend line for the balance history, muted so it supports the figure rather
	// than competing with it.
	rule(".acct-spark",
		display("block"),
		width("120px"),
		height("24px"),
		marginTop("0.3rem"),
		opacity("0.85"),
		overflow("visible"),
	)
	// The ranged balance chart: the 90-day / 12-month / all-time picker sits above the sparkline
	// and its caption, stacked with a small gap.
	rule(".acct-spark-ranged",
		display("flex"),
		flexDirection("column"),
		alignItems("flex-start"),
		gap("0.4rem"),
		minWidth("0"),
		maxWidth("100%"),
	)
	rule(".acct-spark-range-picker",
		maxWidth("100%"),
		minWidth("0"),
	)

	// --- account row: calm, scannable hierarchy ---
	// The row stacks two lines. The primary line holds the name, the balance and the actions.
	// The quiet secondary line holds the type and currency meta and the Details toggle.
	// Everything else sits in the details panel, so the resting list reads name then balance.
	// This overrides .row's horizontal flex, since the row stacks its head and details.
	rule(".acct-row",
		flexDirection("column"),
		alignItems("stretch"),
		justifyContent("flex-start"),
		gap("0"),
	)
	// PRIMARY line: type glyph, identity column (grows), the balance figure, the actions.
	rule(".acct-row-head",
		display("flex"),
		alignItems("center"),
		gap("var(--acct-head-gap, 0.75rem)"),
		width("100%"),
	)
	// Identity column: the name line and the quiet sub line share one row, so the meta sits
	// beside the name and only wraps underneath when the pane is too narrow for both. min-width:0
	// lets long names truncate instead of shoving the figure off the row.
	rule(".acct-row-id",
		display("flex"),
		flexDirection("row"),
		flexWrap("wrap"),
		alignItems("center"),
		prop("column-gap", "0.75rem"),
		rowGap("0.15rem"),
		minWidth("0"),
		flex("1"),
	)
	// Name line: the account name with its status badges and institution chip. Items wrap
	// when the row is narrow.
	rule(".acct-row-name",
		display("flex"),
		alignItems("center"),
		flexWrap("wrap"),
		gap("0.5rem"),
		minWidth("0"),
	)
	// Quiet secondary line: the type · currency meta and the Details disclosure only.
	rule(".acct-row-sub",
		display("flex"),
		alignItems("center"),
		flexWrap("wrap"),
		gap("0.6rem"),
		minWidth("0"),
	)
	// Every row reserves the recurring-charges line, so rows with and without charges match in height.
	rule(".acct-recur",
		minHeight("calc(var(--type-14) * 1.6)"),
	)
	// Comfortable reading size for the account row's secondary text, all from the type scale:
	// the meta line and toggles at the body step, the badges one step up from the caption, and
	// the name at the card-title step so it leads the row.
	rule(".acct-row .row-desc",
		fontSize("var(--type-16)"),
	)
	rule(".acct-row .row-meta, .acct-row .acct-details-toggle, .acct-row .budget-drivers-toggle",
		fontSize("var(--type-14)"),
	)
	rule(".acct-row .badge",
		fontSize("var(--type-13)"),
	)
	rule(".acct-details-chev",
		transition("transform 150ms ease"),
	)
	// The chevron turns to point up while the details are open.
	rule(".acct-details-toggle[aria-expanded=\"true\"] .acct-details-chev",
		transform("rotate(180deg)"),
	)
	// The Details toggle stays on one line. Its font size comes from the .acct-row rule above.
	rule(".acct-details-toggle",
		whiteSpace("nowrap"),
	)
	// The balance figure: tabular, right-aligned, the row's second anchor after the name.
	rule(".acct-row-figure",
		textAlign("right"),
		whiteSpace("nowrap"),
		fontVariantNumeric("tabular-nums"),
		fontSize("var(--type-16)"),
	)
	// Actions cluster: compact, never shrinking, so the buttons stay on one tidy line.
	rule(".acct-row-actions",
		display("inline-flex"),
		alignItems("center"),
		gap("0.4rem"),
		flexShrink("0"),
	)
	// Compact panes: the head row wraps instead of letting the never-shrinking
	// actions cluster spill past the pane edge — the cluster drops to its own
	// right-aligned line under the figure (its margin-left:auto is a no-op
	// while everything still fits on one line, since the identity column
	// already absorbs the slack).
	ruleContentMax(contentGrid4, ".acct-row-head",
		flexWrap("wrap"),
		rowGap("0.35rem"),
	)
	ruleContentMax(contentGrid4, ".acct-row-actions",
		marginLeft("auto"),
	)

	// --- sweep-rules add form: one field per line ---
	// A framed section that groups the add-a-rule controls. Each control stacks its label
	// over the control, with standard field spacing.
	rule(".sweep-add",
		display("flex"),
		flexDirection("column"),
		gap("0.75rem"),
		padding("0.9rem"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-lg)"),
		background("var(--bg-elev)"),
	)
	// Mouse and programmatic focus on a select or field in the sweep-rules form gets a plain
	// border and no glow. Keyboard focus (:focus-visible) keeps the strong accent ring.
	rule("#sweep-rules-form select:focus:not(:focus-visible), #sweep-rules-form .field:focus:not(:focus-visible)",
		borderColor("var(--border)"),
		boxShadow("none"),
	)

	// The reconcile-to-statement flow rules (sticky math header, standing explanation,
	// Mark all cleared row) and the stale-account rules (dot and summary line).
	registerReconcileFlow()
	registerDetail1Accounts()
}
