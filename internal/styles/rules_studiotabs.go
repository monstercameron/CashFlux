// SPDX-License-Identifier: MIT

//go:build js && wasm

package styles

// registerStudioTabs emits the bespoke Studio Manage/Pages tab surfaces —
// from-scratch decks in the studio design language (the Design tab's eyebrow +
// serif masthead classes are reused from the generated rules; everything here
// is the tabs' own vocabulary). Registered after the generated rules so these
// win equal-specificity ties.
func registerStudioTabs() {
	const mono = "ui-monospace, SFMono-Regular, Menlo, monospace"

	// ── Manage: the arrangement deck ─────────────────────────────────────────────
	rule(".wman",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "1.5rem"),
		prop("margin-top", "1rem"),
	)
	rule(".wman-head",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.25rem"),
	)
	rule(".wman-grid",
		prop("display", "grid"),
		prop("grid-template-columns", "minmax(0, 1fr) 23rem"),
		prop("gap", "2.25rem"),
		prop("align-items", "start"),
	)
	ruleContentMax(contentTwoCol, ".wman-grid",
		prop("display", "flex"),
		prop("flex-direction", "column-reverse"),
	)
	rule(".wman-toolbar",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("flex-wrap", "wrap"),
		prop("gap", "0.6rem"),
		prop("padding-bottom", "0.85rem"),
		prop("border-bottom", "1px solid var(--border)"),
	)
	rule(".wman-count",
		prop("margin-left", "auto"),
		prop("font-family", mono),
		prop("font-size", "0.74rem"),
		prop("opacity", "0.6"),
	)

	// The ledger: order number · name · size · reorder · visibility.
	rule(".wman-row",
		prop("display", "grid"),
		prop("grid-template-columns", "2rem minmax(0, 1fr) auto auto auto"),
		prop("gap", "0.9rem"),
		prop("align-items", "center"),
		prop("padding", "0.5rem 0.35rem"),
		prop("border-bottom", "1px solid color-mix(in srgb, var(--text) 8%, transparent)"),
		prop("border-radius", "8px"),
		prop("transition", "background 400ms ease"),
	)
	rule(".wman-row.is-flash",
		prop("background", "color-mix(in srgb, var(--accent) 14%, transparent)"),
	)
	rule(".wman-ord",
		prop("font-family", mono),
		prop("font-size", "0.72rem"),
		prop("opacity", "0.5"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	rule(".wman-id",
		prop("display", "flex"),
		prop("align-items", "baseline"),
		prop("gap", "0.5rem"),
		prop("min-width", "0"),
	)
	rule(".wman-id .wm-name",
		prop("font-weight", "600"),
		prop("overflow", "hidden"),
		prop("text-overflow", "ellipsis"),
		prop("white-space", "nowrap"),
	)
	rule(".wman-row.is-hidden .wman-id",
		prop("opacity", "0.5"),
	)
	rule(".wman-hidden-tag",
		prop("font-size", "0.6rem"),
		prop("font-weight", "600"),
		prop("letter-spacing", "0.08em"),
		prop("text-transform", "uppercase"),
		prop("border", "1px dashed var(--border)"),
		prop("border-radius", "999px"),
		prop("padding", "0.05rem 0.45rem"),
		prop("opacity", "0.7"),
		prop("white-space", "nowrap"),
	)
	// The reorder arrows read faint at rest and full on row hover (the generated
	// .wm-reorder reveal keys on .wm-row:hover already; this keeps a visible
	// affordance instead of a fully empty cell).
	rule(".wman-reorder",
		prop("opacity", "0.35"),
		prop("display", "flex"),
		prop("gap", "2px"),
	)

	// Phones: the row wraps — order + name + switch on the first line, size and
	// reorder controls beneath — and the hover-revealed controls show at rest
	// (there is no hover to reveal them on touch).
	ruleMedia("(max-width: 640px)", ".wman-row",
		prop("grid-template-columns", "2rem minmax(0, 1fr) auto"),
		prop("row-gap", "0.35rem"),
	)
	ruleMedia("(max-width: 640px)", ".wman-row .wm-col-size",
		prop("grid-column", "2"),
	)
	ruleMedia("(max-width: 640px)", ".wman-reorder",
		prop("grid-column", "3"),
		prop("justify-self", "end"),
		prop("opacity", "1"),
	)
	ruleMedia("(max-width: 640px)", ".wman-row .wm-size",
		prop("opacity", "1"),
	)
	ruleMedia("(max-width: 640px)", ".wman-row .wm-static",
		prop("display", "none"),
	)

	// The board map: a miniature 4-column dashboard at true spans.
	rule(".wman-aside",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.6rem"),
		prop("position", "sticky"),
		prop("top", "1rem"),
	)
	ruleContentMax(contentTwoCol, ".wman-aside",
		prop("position", "static"),
	)
	rule(".wman-aside-label",
		prop("font-size", "0.72rem"),
		prop("font-weight", "600"),
		prop("letter-spacing", "0.14em"),
		prop("text-transform", "uppercase"),
		prop("color", "var(--accent)"),
	)
	rule(".wman-map",
		prop("display", "grid"),
		prop("grid-template-columns", "repeat(4, minmax(0, 1fr))"),
		prop("grid-auto-rows", "2.6rem"),
		prop("gap", "5px"),
		prop("padding", "0.6rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "12px"),
		prop("background", "color-mix(in srgb, var(--text) 2.5%, transparent)"),
	)
	rule(".wman-map-tile",
		prop("display", "flex"),
		prop("align-items", "flex-start"),
		prop("padding", "0.3rem 0.4rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "7px"),
		prop("background", "color-mix(in srgb, var(--text) 5%, transparent)"),
		prop("color", "inherit"),
		prop("cursor", "pointer"),
		prop("overflow", "hidden"),
		prop("text-align", "left"),
		prop("min-width", "0"),
	)
	rule(".wman-map-tile:hover",
		prop("border-color", "var(--accent)"),
	)
	rule(".wman-map-tile.is-hidden",
		prop("border-style", "dashed"),
		prop("background", "none"),
		prop("opacity", "0.45"),
	)
	rule(".wman-map-name",
		prop("font-size", "0.58rem"),
		prop("font-weight", "600"),
		prop("letter-spacing", "0.02em"),
		prop("line-height", "1.25"),
		prop("overflow", "hidden"),
		prop("display", "-webkit-box"),
		prop("-webkit-line-clamp", "2"),
		prop("-webkit-box-orient", "vertical"),
	)
	rule(".wman-map-hint",
		prop("font-size", "0.74rem"),
		prop("opacity", "0.55"),
		prop("margin", "0"),
	)

	// ── Formulas / Custom fields tabs: the shared masthead wrapper ───────────────
	rule(".stu-deck",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "1.5rem"),
		prop("margin-top", "1rem"),
	)

	// ── Pages: the page registry + composer rail ─────────────────────────────────
	rule(".spg",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "1.5rem"),
		prop("margin-top", "1rem"),
	)
	rule(".spg-grid",
		prop("display", "grid"),
		prop("grid-template-columns", "minmax(0, 1fr) 23rem"),
		prop("gap", "2.5rem"),
		prop("align-items", "start"),
	)
	ruleContentMax(contentTwoCol, ".spg-grid",
		prop("display", "flex"),
		prop("flex-direction", "column"),
	)
	rule(".spg-reg-head",
		prop("display", "flex"),
		prop("align-items", "baseline"),
		prop("justify-content", "space-between"),
		prop("gap", "1rem"),
		prop("padding-bottom", "0.6rem"),
		prop("border-bottom", "1px solid var(--border)"),
	)
	rule(".spg-empty",
		prop("font-size", "0.88rem"),
		prop("opacity", "0.6"),
		prop("margin", "1rem 0 0"),
	)
	rule(".spg-row",
		prop("display", "grid"),
		prop("grid-template-columns", "minmax(0, 1fr) auto auto"),
		prop("gap", "0.9rem"),
		prop("align-items", "center"),
		prop("padding", "0.8rem 0.2rem"),
		prop("border-bottom", "1px solid color-mix(in srgb, var(--text) 8%, transparent)"),
	)
	rule(".spg-row-top",
		prop("display", "flex"),
		prop("align-items", "baseline"),
		prop("gap", "0.5rem"),
	)
	rule(".spg-name",
		prop("font-family", "var(--font-display, 'Fraunces', serif)"),
		prop("font-size", "1.1rem"),
		prop("font-weight", "600"),
	)
	rule(".spg-row-sub",
		prop("display", "flex"),
		prop("align-items", "baseline"),
		prop("gap", "0.7rem"),
		prop("margin-top", "0.15rem"),
	)
	rule(".spg-slug",
		prop("font-family", mono),
		prop("font-size", "0.76rem"),
		prop("color", "var(--accent)"),
		prop("opacity", "0.9"),
	)
	rule(".spg-meta",
		prop("font-size", "0.76rem"),
		prop("opacity", "0.55"),
	)
	rule(".spg-open",
		prop("font-size", "0.82rem"),
		prop("font-weight", "600"),
		prop("color", "var(--accent)"),
		prop("text-decoration", "none"),
		prop("white-space", "nowrap"),
	)
	rule(".spg-open:hover",
		prop("text-decoration", "underline"),
	)
	rule(".spg-composer",
		prop("position", "sticky"),
		prop("top", "1rem"),
		prop("border-left", "1px solid var(--border)"),
		prop("padding-left", "1.75rem"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
	)
	ruleContentMax(contentTwoCol, ".spg-composer",
		prop("position", "static"),
		prop("border-left", "none"),
		prop("padding-left", "0"),
		prop("max-width", "34rem"),
	)
	rule(".spg-comp-title",
		prop("font-family", "var(--font-display, 'Fraunces', serif)"),
		prop("font-size", "1.15rem"),
		prop("font-weight", "600"),
		prop("margin", "0"),
	)
	rule(".spg-comp-lede",
		prop("font-size", "0.82rem"),
		prop("line-height", "1.5"),
		prop("opacity", "0.65"),
		prop("margin", "0.4rem 0 1rem"),
	)
	rule(".spg-form",
		prop("display", "flex"),
		prop("flex-direction", "column"),
	)
	rule(".spg-create",
		prop("align-self", "stretch"),
	)

	// Registry-row ⋯ menus open leftward: their trigger sits at the row's right
	// edge beside an adjacent rail column, and the default left-aligned menu
	// would spill 210px over the composer (AnchorPopover only flips at viewport
	// edges, not sibling columns). Same for the /fields registry rows.
	rule(".spg-row .add-menu, .fld-row .add-menu",
		prop("left", "auto"),
		prop("right", "0"),
	)

	// Design tab, Custom-layout block editor: the compact selects were the
	// narrowest controls on the busiest row and truncated mid-word ("Figure
	// (a me…"). Let the Shows cell wrap so each control keeps a readable
	// minimum width, and give the per-block doc line the full row.
	rule(".studio-block-shows",
		prop("flex-wrap", "wrap"),
	)
	rule(".studio-block-shows > *",
		prop("flex", "1 1 8.5rem"),
	)
	rule(".studio-block-doc",
		prop("flex", "1 1 100%"),
	)

	// Sections below the fold (tile style studio) speak the serif accent-tick
	// section language.
	rule(".wman-section",
		prop("padding-top", "1.25rem"),
		prop("border-top", "1px solid var(--border)"),
	)
	rule(".wman-section-title",
		prop("font-family", "var(--font-display, 'Fraunces', serif)"),
		prop("font-size", "1.2rem"),
		prop("font-weight", "600"),
		prop("border-left", "3px solid var(--accent)"),
		prop("padding-left", "0.6rem"),
		prop("margin", "0 0 0.35rem"),
	)
	rule(".wman-section-lede",
		prop("font-size", "0.85rem"),
		prop("opacity", "0.65"),
		prop("margin", "0 0 1rem"),
		prop("max-width", "42rem"),
	)
}
