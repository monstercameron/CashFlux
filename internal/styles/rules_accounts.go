// SPDX-License-Identifier: MIT

package styles

// registerAccountsSurface emits the accounts-page refinements: the merged editor's
// "update value" group and the readable, expandable notes line on each account row.
// Registered from Register().
func registerAccountsSurface() {
	// --- merged editor: the "update value / balance" group at the top of the form ---
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

	// --- AC1: account groups (sections + subtotal header) ---
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

	// --- AC2: balance sparkline ---
	// A compact trend line under the account meta; muted so it supports, not competes.
	rule(".acct-spark",
		prop("display", "block"),
		prop("width", "120px"),
		prop("height", "24px"),
		prop("margin-top", "0.3rem"),
		prop("opacity", "0.85"),
		prop("overflow", "visible"),
	)

	// --- AC9: in / out / net flow figures ---
	rule(".acct-flow",
		prop("display", "inline-flex"),
		prop("align-items", "baseline"),
		prop("flex-wrap", "wrap"),
		prop("gap", "0.15rem"),
		prop("font-variant-numeric", "tabular-nums"),
		prop("margin-top", "0.2rem"),
	)
	rule(".acct-flow-net",
		prop("font-weight", "500"),
	)

	// --- account row redesign: calm, scannable hierarchy -----------------------------
	// The row is now vertical: a strong PRIMARY line (name + balance + actions) over a
	// quiet SECONDARY line (meta + a "Details" disclosure). Everything else folds into
	// the details block, so the resting list reads name → balance and nothing competes.
	// Override .row's horizontal flex — the account row stacks its head and details.
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
	// Identity column: name line over the quiet sub line. min-width:0 lets long names
	// truncate instead of shoving the figure off the row.
	rule(".acct-row-id",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.1rem"),
		prop("min-width", "0"),
		prop("flex", "1"),
	)
	// Name line: the account name plus its status badges / institution chip, wrapping
	// gracefully. The 0.5rem gap replaces the old per-badge inline margins.
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
	// The Details toggle is a quiet link — small, dim, never louder than the meta.
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
	// The revealed detail block: indented to sit under the name, quietly separated from
	// the primary line, with comfortable spacing between its stacked items.
	rule(".acct-row-details",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("align-items", "flex-start"),
		prop("gap", "0.4rem"),
		prop("width", "100%"),
		prop("margin-top", "0.55rem"),
		prop("padding", "0.6rem 0 0.15rem 1.9rem"),
		prop("border-top", "1px solid var(--border)"),
	)

	// --- sweep-rules add form: one field per line ------------------------------------
	// A subtly framed section so the "add a rule" controls read as their own group,
	// stacked label-over-control with standard field spacing (fixing the old cramped,
	// wrapping horizontal row and the tall empty modal void).
	rule(".sweep-add",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.75rem"),
		prop("padding", "0.9rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "var(--radius-lg)"),
		prop("background", "var(--bg-elev)"),
	)
	rule(".sweep-add-btn",
		prop("width", "100%"),
		prop("margin-top", "0.15rem"),
	)
	// Soften the autofocus ring: the modal moves focus to the first <select> on open,
	// which fired the bright accent border/box-shadow even for a mouse user who never
	// tabbed there. Drop the loud treatment for programmatic/mouse focus while keeping
	// the strong keyboard ring (:focus-visible) untouched for accessibility.
	rule("#sweep-rules-form select:focus:not(:focus-visible), #sweep-rules-form .field:focus:not(:focus-visible)",
		prop("border-color", "var(--border)"),
		prop("box-shadow", "none"),
	)
}
