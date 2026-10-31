// SPDX-License-Identifier: MIT

package styles

// registerAccountsSurface emits the accounts-page rules: the account editor's value section,
// the account row and its notes line, the account groups, the balance sparkline and the
// sweep-rules add form. It also chains the reconcile-flow and stale-account rules.
// Called from Register in install.go.
func registerAccountsSurface() {
	// The add-account form's full-width rows (the liability and ownership sub-forms, the
	// advanced section) span both columns of the form grid.
	// A name field with a validation message is outlined in the danger colour, so the error reads
	// before the message is read. ClassIf and AttrIf set the marker from the form's error state.
	rule(".field.field-invalid",
		borderColor("var(--danger, #e5484d)"),
	)
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
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.4rem"),
		prop("padding", "0.8rem 0.9rem"),
		prop("margin-bottom", "0.9rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "var(--radius-lg)"),
		prop("background", "var(--hover)"),
	)
	// The "Currently $X" context sits just under the input; the delta preview under that.
	rule(".acct-value-now",
		prop("font-size", "var(--type-13)"),
	)
	rule(".acct-value-delta",
		prop("margin", "0.05rem 0 0"),
	)

	// --- account row: readable, expandable notes line ---
	// The attached note itself, shown as a subtle framed disclosure that clamps to two
	// lines and expands on click — legible at a glance, not hidden in a hover tooltip.
	rule(".acct-notes",
		prop("display", "flex"),
		prop("align-items", "flex-start"),
		prop("gap", "0.4rem"),
		prop("width", "100%"),
		prop("max-width", "42rem"),
		prop("text-align", "left"),
		prop("margin-top", "0.4rem"),
		prop("padding", "0.4rem 0.55rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "var(--radius-lg)"),
		prop("background", "transparent"),
		prop("color", "var(--text-dim)"),
		prop("font-size", "var(--type-13)"),
		prop("line-height", "1.4"),
		prop("cursor", "pointer"),
		prop("transition", "background .15s ease, border-color .15s ease, color .15s ease"),
	)
	rule(".acct-notes:hover",
		prop("background", "var(--hover)"),
		prop("border-color", "var(--text-dim)"),
		prop("color", "var(--text)"),
	)
	rule(".acct-notes:focus-visible",
		prop("outline", "2px solid var(--accent)"),
		prop("outline-offset", "2px"),
	)
	rule(".acct-notes-icon",
		prop("margin-top", "0.12rem"),
		prop("opacity", "0.75"),
	)
	// Collapsed: clamp to two lines with an ellipsis. Preserves the note's own line
	// breaks (white-space:pre-wrap) so a multi-line note still reads naturally.
	rule(".acct-notes-text",
		prop("display", "-webkit-box"),
		prop("-webkit-line-clamp", "2"),
		prop("-webkit-box-orient", "vertical"),
		prop("overflow", "hidden"),
		prop("white-space", "pre-wrap"),
		prop("word-break", "break-word"),
	)
	// Expanded: reveal the whole note.
	rule(".acct-notes.open .acct-notes-text",
		prop("-webkit-line-clamp", "unset"),
		prop("overflow", "visible"),
	)

	// --- account groups: sections with a subtotal header ---
	rule(".acct-group",
		prop("margin-bottom", "0.6rem"),
	)
	// A quiet section header above each group's rows: name on the left, net subtotal on
	// the right, with a subtle rule underneath so the grouping reads without shouting.
	rule(".acct-group-header",
		prop("padding", "0.35rem 0.15rem 0.3rem"),
		prop("margin-top", "0.2rem"),
		prop("border-bottom", "1px solid var(--border)"),
		prop("font-size", "var(--type-14)"),
	)
	rule(".acct-group-name",
		prop("color", "var(--text)"),
	)
	rule(".acct-group-subtotal",
		prop("font-variant-numeric", "tabular-nums"),
	)

	// --- balance sparkline ---
	// A compact trend line for the balance history, muted so it supports the figure rather
	// than competing with it.
	rule(".acct-spark",
		prop("display", "block"),
		prop("width", "120px"),
		prop("height", "24px"),
		prop("margin-top", "0.3rem"),
		prop("opacity", "0.85"),
		prop("overflow", "visible"),
	)

	// --- account row: calm, scannable hierarchy ---
	// The row stacks two lines. The primary line holds the name, the balance and the actions.
	// The quiet secondary line holds the type and currency meta and the Details toggle.
	// Everything else sits in the details panel, so the resting list reads name then balance.
	// This overrides .row's horizontal flex, since the row stacks its head and details.
	rule(".acct-row",
		prop("flex-direction", "column"),
		prop("align-items", "stretch"),
		prop("justify-content", "flex-start"),
		prop("gap", "0"),
	)
	// PRIMARY line: type glyph, identity column (grows), the balance figure, the actions.
	rule(".acct-row-head",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.75rem"),
		prop("width", "100%"),
	)
	// Identity column: the name line and the quiet sub line share one row, so the meta sits
	// beside the name and only wraps underneath when the pane is too narrow for both. min-width:0
	// lets long names truncate instead of shoving the figure off the row.
	rule(".acct-row-id",
		prop("display", "flex"),
		prop("flex-direction", "row"),
		prop("flex-wrap", "wrap"),
		prop("align-items", "center"),
		prop("column-gap", "0.75rem"),
		prop("row-gap", "0.15rem"),
		prop("min-width", "0"),
		prop("flex", "1"),
	)
	// Name line: the account name with its status badges and institution chip. Items wrap
	// when the row is narrow.
	rule(".acct-row-name",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("flex-wrap", "wrap"),
		prop("gap", "0.5rem"),
		prop("min-width", "0"),
	)
	// Quiet secondary line: the type · currency meta and the Details disclosure only.
	rule(".acct-row-sub",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("flex-wrap", "wrap"),
		prop("gap", "0.6rem"),
		prop("min-width", "0"),
	)
	// Every row reserves the recurring-charges line, so rows with and without charges match in height.
	rule(".acct-recur",
		prop("min-height", "calc(var(--type-14) * 1.6)"),
	)
	// Comfortable reading size for the account row's secondary text, all from the type scale:
	// the meta line and toggles at the body step, the badges one step up from the caption, and
	// the name at the card-title step so it leads the row.
	rule(".acct-row .row-desc",
		prop("font-size", "var(--type-16)"),
	)
	rule(".acct-row .row-meta, .acct-row .acct-details-toggle, .acct-row .budget-drivers-toggle",
		prop("font-size", "var(--type-14)"),
	)
	rule(".acct-row .badge",
		prop("font-size", "var(--type-13)"),
	)
	rule(".acct-details-chev",
		prop("transition", "transform 150ms ease"),
	)
	// The chevron turns to point up while the details are open.
	rule(".acct-details-toggle[aria-expanded=\"true\"] .acct-details-chev",
		prop("transform", "rotate(180deg)"),
	)
	// The Details toggle stays on one line.
	rule(".acct-details-toggle",
		prop("font-size", "var(--type-13)"),
		prop("white-space", "nowrap"),
	)
	// The balance figure: tabular, right-aligned, the row's second anchor after the name.
	rule(".acct-row-figure",
		prop("text-align", "right"),
		prop("white-space", "nowrap"),
		prop("font-variant-numeric", "tabular-nums"),
		prop("font-size", "var(--type-16)"),
	)
	// Actions cluster: compact, never shrinking, so the buttons stay on one tidy line.
	rule(".acct-row-actions",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("gap", "0.4rem"),
		prop("flex-shrink", "0"),
	)
	// Compact panes: the head row wraps instead of letting the never-shrinking
	// actions cluster spill past the pane edge — the cluster drops to its own
	// right-aligned line under the figure (its margin-left:auto is a no-op
	// while everything still fits on one line, since the identity column
	// already absorbs the slack).
	ruleContentMax(contentGrid4, ".acct-row-head",
		prop("flex-wrap", "wrap"),
		prop("row-gap", "0.35rem"),
	)
	ruleContentMax(contentGrid4, ".acct-row-actions",
		prop("margin-left", "auto"),
	)

	// --- sweep-rules add form: one field per line ---
	// A framed section that groups the add-a-rule controls. Each control stacks its label
	// over the control, with standard field spacing.
	rule(".sweep-add",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.75rem"),
		prop("padding", "0.9rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "var(--radius-lg)"),
		prop("background", "var(--bg-elev)"),
	)
	// Mouse and programmatic focus on a select or field in the sweep-rules form gets a plain
	// border and no glow. Keyboard focus (:focus-visible) keeps the strong accent ring.
	rule("#sweep-rules-form select:focus:not(:focus-visible), #sweep-rules-form .field:focus:not(:focus-visible)",
		prop("border-color", "var(--border)"),
		prop("box-shadow", "none"),
	)

	// The reconcile-to-statement flow rules (sticky math header, standing explanation,
	// Mark all cleared row) and the stale-account rules (dot and summary line).
	registerReconcileFlow()
	registerDetail1Accounts()
}
