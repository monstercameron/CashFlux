// SPDX-License-Identifier: MIT

package styles

// registerHouseholdSurface emits the /household (and /members) people-ledger
// design: the auto-row bento host, the person roster rows (oversized avatar,
// serif name, paired worth/spent figure column, net-worth share bar), and the
// quiet chip row. Hero/section/takeaway chrome reuses the shared rpt-*/debt-*
// rules so the page reads as a sibling of the Understand surfaces; everything
// is token-based so it tracks every theme. Registered from Register().
func registerHouseholdSurface() {
	rule(".bento.bento-house",
		prop("grid-template-rows", "auto"),
		prop("grid-auto-rows", "auto"),
	)
	rule(".bento.bento-house > .w",
		prop("height", "auto"),
		prop("min-height", "0"),
		prop("overflow", "visible"),
	)

	// ── Person roster row ────────────────────────────────────────────────────
	// A person is a small ledger entry: identity on the left, standing on the
	// right, their slice of the household underneath.
	// The .row base class centers/space-betweens its children — restate the
	// column axis so the person row stacks full-width, top-aligned.
	rule(".row.hh-person",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("align-items", "stretch"),
		prop("justify-content", "flex-start"),
		prop("gap", "0.45rem"),
		prop("padding", "0.85rem 0"),
		prop("border-bottom", "1px solid var(--border)"),
	)
	rule(".hh-person:last-child", prop("border-bottom", "none"))
	rule(".hh-person-main",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.85rem"),
		prop("flex-wrap", "wrap"),
		prop("width", "100%"),
	)
	// Identity block: avatar + serif name + chips.
	rule(".hh-person-id",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.75rem"),
		prop("min-width", "0"),
		prop("flex", "1 1 14rem"),
	)
	// The roster avatar is deliberately oversized relative to the app's 1.5rem
	// discs — on the people page, the person IS the subject.
	rule(".hh-person .member-avatar",
		prop("width", "2.4rem"),
		prop("height", "2.4rem"),
		prop("font-size", "1.05rem"),
		prop("margin-right", "0"),
	)
	rule(".hh-person-name",
		prop("font-size", "1.1rem"),
		prop("font-weight", "600"),
		prop("line-height", "1.2"),
		prop("overflow", "hidden"),
		prop("text-overflow", "ellipsis"),
		prop("white-space", "nowrap"),
	)
	rule(".hh-person-chips",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.35rem"),
		prop("flex-wrap", "wrap"),
		prop("margin-top", "0.2rem"),
	)
	// Figure column: net worth over a quiet spent-this-period sub-line, right-aligned.
	rule(".hh-person-figures",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("align-items", "flex-end"),
		prop("gap", "0.1rem"),
		prop("margin-left", "auto"),
		prop("flex-shrink", "0"),
	)
	rule(".hh-person-worth",
		prop("font-size", "1.25rem"),
		prop("font-weight", "700"),
		prop("line-height", "1.1"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	rule(".hh-person-sub",
		prop("font-size", "0.75rem"),
		prop("color", "var(--text-dim)"),
		prop("font-variant-numeric", "tabular-nums"),
		prop("white-space", "nowrap"),
	)
	// Row actions sit after the figures; quiet until the row is hovered.
	rule(".hh-person-actions",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.35rem"),
	)
	// The person's slice of household net worth. Indented past the avatar so the
	// bars align into a scannable column across rows.
	rule(".hh-person-share",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.6rem"),
		prop("padding-left", "3.15rem"),
	)
	rule(".hh-person-share .share-bar",
		prop("flex", "1"),
		prop("margin", "0"),
	)
	rule(".hh-person-share-pct",
		prop("font-size", "0.72rem"),
		prop("color", "var(--text-dim)"),
		prop("font-variant-numeric", "tabular-nums"),
		prop("min-width", "2.6rem"),
		prop("text-align", "right"),
	)
	// Custom-field values read as a quiet meta line under the identity.
	rule(".hh-person-custom",
		prop("font-size", "0.75rem"),
		prop("color", "var(--text-dim)"),
		prop("padding-left", "3.15rem"),
	)
	// Inline edit / PIN forms open indented under the row main line. Selects
	// get room to show "Inherit household default" without clipping into the
	// caret glyph.
	rule(".hh-person-form",
		prop("padding", "0.35rem 0 0.25rem 3.15rem"),
	)
	rule(".hh-person-form select.field",
		prop("min-width", "15rem"),
		prop("padding-right", "1.75rem"),
		prop("text-overflow", "ellipsis"),
	)
	// On narrow screens the indent collapses so forms and bars use full width.
	ruleMedia("(max-width:40rem)", ".hh-person-share", prop("padding-left", "0"))
	ruleMedia("(max-width:40rem)", ".hh-person-custom", prop("padding-left", "0"))
	ruleMedia("(max-width:40rem)", ".hh-person-form", prop("padding-left", "0"))

	// ── Split panel ──────────────────────────────────────────────────────────
	// The split calculator's key output — the per-person sentence — reads as a
	// serif pull-quote, matching the takeaway voice of the sibling surfaces.
	rule(".bento-house .split-summary",
		prop("font-size", "1.05rem"),
		prop("font-style", "italic"),
		prop("border-left", "2px solid var(--accent)"),
		prop("padding-left", "0.85rem"),
		prop("margin", "0.85rem 0 0.25rem"),
	)
}
