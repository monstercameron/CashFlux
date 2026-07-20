// SPDX-License-Identifier: MIT

package styles

// registerRhythmSurface emits the from-scratch "month's rhythm" vocabulary for
// the unified Bills & Recurring surface (/recurring, /bills, /subscriptions).
// The prefix is `rhy-` and the design is deliberately NOT the bento tile kit: a
// stack of full-width sections — a tideline hero, an overdue strip, a review
// strip, the up-next agenda (compact | calendar), the lineup roster, and a
// findings strip. Theme tokens only (--text/--text-dim/--border/--bg-card/--bg/
// --accent, plus --danger/--warn reserved for overdue and a negative pinch);
// --radius is never redefined. Registered from Register().
func registerRhythmSurface() {
	// ── Page shell: a vertical stack of sections filling the content column. ──
	rule(".rhy",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "1.15rem"),
		prop("width", "100%"),
	)
	rule(".rhy-section",
		prop("background", "var(--bg-card)"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "var(--radius)"),
		prop("padding", "1.1rem 1.15rem"),
	)
	// The hero and the strips read as one field, no card seam between them.
	rule(".rhy-section.rhy-flush",
		prop("padding", "0"),
		prop("border", "0"),
		prop("background", "transparent"),
	)
	rule(".rhy-sec-head",
		prop("display", "flex"),
		prop("align-items", "baseline"),
		prop("justify-content", "space-between"),
		prop("gap", "0.75rem"),
		prop("flex-wrap", "wrap"),
		prop("margin-bottom", "0.6rem"),
	)
	rule(".rhy-sec-title",
		prop("font-size", "var(--type-16)"),
		prop("font-weight", "650"),
		prop("letter-spacing", "-0.01em"),
		prop("margin", "0"),
	)
	rule(".rhy-sec-note",
		prop("color", "var(--text-dim)"),
		prop("font-size", "var(--type-13)"),
		prop("margin", "0 0 0.6rem"),
	)

	registerRhythmTideline()
	registerRhythmStrips()
	registerRhythmAgenda()
	registerRhythmRoster()
	registerRhythmBreakpoints()
}

// registerRhythmTideline emits the signature hero: an SVG pay-cycle band plus a
// compact stat rail.
func registerRhythmTideline() {
	rule(".rhy-hero",
		prop("display", "grid"),
		prop("grid-template-columns", "minmax(0, 1fr) auto"),
		prop("gap", "1.25rem"),
		prop("align-items", "stretch"),
	)
	rule(".rhy-tide",
		prop("position", "relative"),
		prop("min-width", "0"),
	)
	rule(".rhy-tide-svg",
		prop("display", "block"),
		prop("width", "100%"),
		prop("height", "auto"),
		prop("overflow", "visible"),
	)
	// Income up-ticks carry the accent; outflow down-ticks are a muted line.
	rule(".rhy-tick-in",
		prop("stroke", "var(--accent)"),
		prop("stroke-width", "2"),
		prop("stroke-linecap", "round"),
	)
	rule(".rhy-tick-in.is-scheduled",
		prop("stroke-dasharray", "2 2"),
	)
	rule(".rhy-tick-out",
		prop("stroke", "var(--text-dim)"),
		prop("stroke-width", "2"),
		prop("stroke-linecap", "round"),
		prop("opacity", "0.7"),
	)
	rule(".rhy-tick-out.is-scheduled",
		prop("opacity", "0.4"),
	)
	rule(".rhy-cushion",
		prop("fill", "none"),
		prop("stroke", "var(--text)"),
		prop("stroke-width", "1.5"),
		prop("opacity", "0.55"),
	)
	rule(".rhy-cushion-area",
		prop("fill", "var(--accent)"),
		prop("opacity", "0.06"),
	)
	rule(".rhy-axis",
		prop("stroke", "var(--border)"),
		prop("stroke-width", "1"),
	)
	rule(".rhy-axis-label",
		prop("fill", "var(--text-dim)"),
		prop("font-size", "10px"),
	)
	rule(".rhy-today",
		prop("stroke", "var(--text)"),
		prop("stroke-width", "1"),
		prop("stroke-dasharray", "3 3"),
		prop("opacity", "0.5"),
	)
	// The pinch marker + flag. Amber by default; red only when negative.
	rule(".rhy-pinch-dot",
		prop("fill", "var(--warn)"),
		prop("stroke", "var(--bg-card)"),
		prop("stroke-width", "1.5"),
	)
	rule(".rhy-pinch-dot.is-neg",
		prop("fill", "var(--danger)"),
	)
	rule(".rhy-tick-hit",
		prop("cursor", "pointer"),
	)
	rule(".rhy-tide-svg .rhy-tick-hit:hover + .rhy-tick-in",
		prop("stroke-width", "3"),
	)
	// Pinch caption under the band.
	rule(".rhy-pinch-note",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("gap", "0.4rem"),
		prop("margin-top", "0.5rem"),
		prop("padding", "0.28rem 0.6rem"),
		prop("border-radius", "999px"),
		prop("font-size", "var(--type-13)"),
		prop("background", "color-mix(in srgb, var(--warn) 14%, transparent)"),
		prop("color", "var(--text)"),
		prop("border", "1px solid color-mix(in srgb, var(--warn) 40%, var(--border))"),
	)
	rule(".rhy-pinch-note.is-neg",
		prop("background", "color-mix(in srgb, var(--danger) 14%, transparent)"),
		prop("border-color", "color-mix(in srgb, var(--danger) 45%, var(--border))"),
	)
	rule(".rhy-tide-empty",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("justify-content", "center"),
		prop("gap", "0.5rem"),
		prop("min-height", "6rem"),
		prop("color", "var(--text-dim)"),
	)
	// Compact stat rail beside the band.
	rule(".rhy-stats",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("justify-content", "center"),
		prop("gap", "0.85rem"),
		prop("min-width", "9rem"),
		prop("padding-left", "1.1rem"),
		prop("border-left", "1px solid var(--border)"),
	)
	rule(".rhy-stat",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.1rem"),
	)
	rule(".rhy-stat-label",
		prop("font-size", "var(--type-11)"),
		prop("color", "var(--text-dim)"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.04em"),
	)
	rule(".rhy-stat-value",
		prop("font-size", "var(--type-20)"),
		prop("font-weight", "650"),
		prop("font-variant-numeric", "tabular-nums"),
	)
}

// registerRhythmStrips emits the overdue, review, and findings strips plus the
// shared row/badge/tag primitives.
func registerRhythmStrips() {
	// Posting-mode + evidence badges, shared across sections.
	rule(".rhy-badge",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("gap", "0.25rem"),
		prop("font-size", "var(--type-11)"),
		prop("font-weight", "600"),
		prop("padding", "0.08rem 0.4rem"),
		prop("border-radius", "999px"),
		prop("border", "1px solid var(--border)"),
		prop("color", "var(--text-dim)"),
		prop("white-space", "nowrap"),
	)
	rule(".rhy-badge.is-auto",
		prop("border-color", "color-mix(in srgb, var(--accent) 45%, var(--border))"),
		prop("color", "var(--accent)"),
	)
	rule(".rhy-badge.is-watch",
		prop("border-style", "dashed"),
	)

	// Overdue strip — the one place (with a negative pinch) red is allowed.
	rule(".rhy-overdue",
		prop("border-color", "color-mix(in srgb, var(--danger) 40%, var(--border))"),
		prop("background", "color-mix(in srgb, var(--danger) 6%, var(--bg-card))"),
	)
	rule(".rhy-overdue-head",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.5rem"),
		prop("font-weight", "650"),
		prop("color", "var(--danger)"),
		prop("margin-bottom", "0.6rem"),
	)
	rule(".rhy-row",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.75rem"),
		prop("padding", "0.5rem 0"),
		prop("border-top", "1px solid var(--border)"),
	)
	rule(".rhy-row:first-of-type",
		prop("border-top", "0"),
	)
	rule(".rhy-row-main",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.1rem"),
		prop("min-width", "0"),
		prop("flex", "1 1 auto"),
	)
	rule(".rhy-row-name",
		prop("font-weight", "600"),
		prop("overflow", "hidden"),
		prop("text-overflow", "ellipsis"),
		prop("white-space", "nowrap"),
	)
	rule(".rhy-row-meta",
		prop("font-size", "var(--type-12)"),
		prop("color", "var(--text-dim)"),
	)
	rule(".rhy-row-amt",
		prop("font-variant-numeric", "tabular-nums"),
		prop("font-weight", "600"),
		prop("white-space", "nowrap"),
	)
	rule(".rhy-row-actions",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.35rem"),
		prop("flex-shrink", "0"),
	)

	// Review strip — the provenance trust ladder.
	rule(".rhy-review-group + .rhy-review-group",
		prop("margin-top", "0.9rem"),
		prop("padding-top", "0.9rem"),
		prop("border-top", "1px solid var(--border)"),
	)
	rule(".rhy-group-head",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.45rem"),
		prop("font-size", "var(--type-13)"),
		prop("font-weight", "600"),
		prop("margin-bottom", "0.5rem"),
	)
	rule(".rhy-smark",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("gap", "0.25rem"),
		prop("font-size", "var(--type-11)"),
		prop("font-weight", "700"),
		prop("padding", "0.05rem 0.4rem"),
		prop("border-radius", "999px"),
		prop("color", "var(--accent)"),
		prop("background", "color-mix(in srgb, var(--accent) 12%, transparent)"),
	)
	rule(".rhy-smark.is-plus",
		prop("color", "var(--text)"),
		prop("background", "color-mix(in srgb, var(--accent) 22%, transparent)"),
		prop("border", "1px solid color-mix(in srgb, var(--accent) 45%, var(--border))"),
	)
	rule(".rhy-cand",
		prop("padding", "0.55rem 0"),
		prop("border-top", "1px solid var(--border)"),
	)
	rule(".rhy-cand:first-of-type",
		prop("border-top", "0"),
	)
	rule(".rhy-cand-top",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.6rem"),
	)
	rule(".rhy-cand-name",
		prop("font-weight", "600"),
		prop("flex", "1 1 auto"),
		prop("min-width", "0"),
	)
	rule(".rhy-cand-ev",
		prop("font-size", "var(--type-12)"),
		prop("color", "var(--text-dim)"),
		prop("margin", "0.2rem 0 0"),
	)
	rule(".rhy-cand-reason",
		prop("font-size", "var(--type-12)"),
		prop("color", "var(--text-dim)"),
		prop("font-style", "italic"),
		prop("margin", "0.2rem 0 0"),
	)
	rule(".rhy-ev-list",
		prop("margin", "0.35rem 0 0"),
		prop("padding", "0.4rem 0.6rem"),
		prop("list-style", "none"),
		prop("background", "var(--bg)"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "var(--radius)"),
		prop("font-size", "var(--type-12)"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	rule(".rhy-ev-list li",
		prop("display", "flex"),
		prop("justify-content", "space-between"),
		prop("gap", "0.75rem"),
		prop("padding", "0.12rem 0"),
		prop("color", "var(--text-dim)"),
	)
	// The paged candidate region is bounded so the review strip can never
	// dominate the viewport, whatever the page size or how many the detector
	// found. The pager + opt-in footer sit OUTSIDE this scroll region.
	rule(".rhy-review-page",
		prop("max-height", "30rem"),
		prop("overflow-y", "auto"),
	)
	rule(".rhy-review-foot",
		prop("margin-top", "0.85rem"),
		prop("padding-top", "0.75rem"),
		prop("border-top", "1px solid var(--border)"),
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.6rem"),
		prop("flex-wrap", "wrap"),
	)

	// Findings strip.
	rule(".rhy-finding",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.6rem"),
		prop("padding", "0.5rem 0"),
		prop("border-top", "1px solid var(--border)"),
	)
	rule(".rhy-finding:first-of-type",
		prop("border-top", "0"),
	)
	rule(".rhy-finding-ic",
		prop("flex-shrink", "0"),
		prop("color", "var(--warn)"),
	)
	rule(".rhy-finding-ic.is-alarm",
		prop("color", "var(--danger)"),
	)
	rule(".rhy-finding-text",
		prop("flex", "1 1 auto"),
		prop("min-width", "0"),
		prop("font-size", "var(--type-13)"),
	)

	// Utilities toolbar.
	rule(".rhy-tools",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.5rem"),
		prop("flex-wrap", "wrap"),
	)
	rule(".rhy-tools-spacer",
		prop("flex", "1 1 auto"),
	)
}

// registerRhythmAgenda emits the up-next agenda: the view toggle plus the dense
// compact list. The CALENDAR view reuses the shared .cal-grid rules from the
// bills surface (kept intact), so only the compact list needs new chrome.
func registerRhythmAgenda() {
	rule(".rhy-agenda-list",
		prop("display", "flex"),
		prop("flex-direction", "column"),
	)
	rule(".rhy-ag-row",
		prop("display", "grid"),
		prop("grid-template-columns", "4.2rem minmax(0, 1fr) auto auto"),
		prop("align-items", "center"),
		prop("gap", "0.7rem"),
		prop("padding", "0.4rem 0"),
		prop("border-top", "1px solid var(--border)"),
	)
	rule(".rhy-ag-row:first-of-type",
		prop("border-top", "0"),
	)
	rule(".rhy-ag-row.is-overdue .rhy-ag-date",
		prop("color", "var(--danger)"),
	)
	rule(".rhy-ag-date",
		prop("font-size", "var(--type-12)"),
		prop("font-weight", "600"),
		prop("color", "var(--text-dim)"),
		prop("white-space", "nowrap"),
	)
	rule(".rhy-ag-body",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.45rem"),
		prop("min-width", "0"),
	)
	rule(".rhy-ag-name",
		prop("font-weight", "550"),
		prop("overflow", "hidden"),
		prop("text-overflow", "ellipsis"),
		prop("white-space", "nowrap"),
	)
	rule(".rhy-ag-amt",
		prop("font-variant-numeric", "tabular-nums"),
		prop("font-weight", "600"),
		prop("white-space", "nowrap"),
		prop("text-align", "right"),
	)
	rule(".rhy-ag-verb",
		prop("display", "flex"),
		prop("justify-content", "flex-end"),
		prop("min-width", "0"),
	)
	// Density: when the app is in compact density, tighten the row rhythm.
	rule("[data-density=\"compact\"] .rhy-ag-row",
		prop("padding", "0.25rem 0"),
	)
	// CALENDAR view: the same agenda data as a month grid, carrying REAL amounts
	// on the days (never bare dots) with income distinguished by the accent.
	rule(".rhy-cal-cell",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("align-items", "flex-start"),
		prop("gap", "0.1rem"),
		prop("min-height", "4.4rem"),
		prop("padding", "0.25rem 0.3rem"),
		prop("overflow", "hidden"),
	)
	rule(".rhy-cal-day",
		prop("font-size", "var(--type-11)"),
		prop("color", "var(--text-dim)"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	rule(".rhy-cal-cell.today .rhy-cal-day",
		prop("color", "var(--text)"),
		prop("font-weight", "700"),
	)
	rule(".rhy-cal-amt",
		prop("display", "block"),
		prop("max-width", "100%"),
		prop("font-size", "var(--type-11)"),
		prop("font-variant-numeric", "tabular-nums"),
		prop("color", "var(--text-dim)"),
		prop("white-space", "nowrap"),
		prop("overflow", "hidden"),
		prop("text-overflow", "ellipsis"),
	)
	rule(".rhy-cal-amt.is-in",
		prop("color", "var(--accent)"),
		prop("font-weight", "600"),
	)
	rule(".rhy-cal-more",
		prop("font-size", "var(--type-11)"),
		prop("color", "var(--text-dim)"),
		prop("opacity", "0.8"),
	)
	rule(".rhy-ag-fit",
		prop("font-size", "var(--type-11)"),
		prop("color", "var(--text-dim)"),
	)
	rule(".rhy-ag-fit.is-over",
		prop("color", "var(--warn)"),
	)
}

// registerRhythmRoster emits the lineup roster: lenses, the weight-first claim
// rows with a %-of-outflow spine, chips, and the watching-after-cancellation
// tail group.
func registerRhythmRoster() {
	rule(".rhy-lenses",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.3rem"),
		prop("flex-wrap", "wrap"),
	)
	rule(".rhy-lens",
		prop("padding", "0.22rem 0.65rem"),
		prop("border-radius", "999px"),
		prop("border", "1px solid var(--border)"),
		prop("background", "transparent"),
		prop("color", "var(--text-dim)"),
		prop("font-size", "var(--type-12)"),
		prop("cursor", "pointer"),
	)
	rule(".rhy-lens.is-on",
		prop("background", "var(--accent)"),
		prop("border-color", "var(--accent)"),
		prop("color", "var(--accent-fg)"),
	)
	rule(".rhy-lens-sub",
		prop("margin-left", "auto"),
		prop("font-size", "var(--type-12)"),
		prop("color", "var(--text-dim)"),
	)
	rule(".rhy-roster-list",
		prop("display", "flex"),
		prop("flex-direction", "column"),
	)
	rule(".rhy-claim",
		prop("display", "grid"),
		prop("grid-template-columns", "8rem minmax(0, 1fr) auto auto"),
		prop("align-items", "center"),
		prop("gap", "0.75rem"),
		prop("padding", "0.55rem 0"),
		prop("border-top", "1px solid var(--border)"),
	)
	rule(".rhy-claim:first-of-type",
		prop("border-top", "0"),
	)
	// The scannable weight spine: a share-of-outflow bar under a tiny percent.
	rule(".rhy-spine",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.2rem"),
	)
	rule(".rhy-spine-pct",
		prop("font-size", "var(--type-11)"),
		prop("color", "var(--text-dim)"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	rule(".rhy-spine-track",
		prop("height", "0.35rem"),
		prop("border-radius", "999px"),
		prop("background", "var(--bg)"),
		prop("overflow", "hidden"),
	)
	rule(".rhy-spine-fill",
		prop("height", "100%"),
		prop("border-radius", "999px"),
		prop("background", "var(--text-dim)"),
		prop("opacity", "0.55"),
	)
	rule(".rhy-claim-main",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.15rem"),
		prop("min-width", "0"),
	)
	rule(".rhy-claim-head",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.4rem"),
		prop("flex-wrap", "wrap"),
	)
	rule(".rhy-claim-name",
		prop("font-weight", "600"),
	)
	rule(".rhy-claim-meta",
		prop("font-size", "var(--type-12)"),
		prop("color", "var(--text-dim)"),
	)
	rule(".rhy-chip",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("gap", "0.2rem"),
		prop("font-size", "var(--type-11)"),
		prop("padding", "0.05rem 0.4rem"),
		prop("border-radius", "999px"),
		prop("border", "1px solid var(--border)"),
		prop("color", "var(--text-dim)"),
	)
	rule(".rhy-chip.is-anchor",
		prop("cursor", "pointer"),
	)
	rule(".rhy-chip.is-creep",
		prop("color", "var(--warn)"),
		prop("border-color", "color-mix(in srgb, var(--warn) 40%, var(--border))"),
	)
	rule(".rhy-claim-amt",
		prop("text-align", "right"),
		prop("min-width", "0"),
	)
	rule(".rhy-claim-per",
		prop("font-weight", "650"),
		prop("font-variant-numeric", "tabular-nums"),
		prop("white-space", "nowrap"),
	)
	rule(".rhy-claim-cad",
		prop("font-size", "var(--type-11)"),
		prop("color", "var(--text-dim)"),
		prop("white-space", "nowrap"),
	)
	rule(".rhy-watch-group",
		prop("margin-top", "0.75rem"),
	)
	rule(".rhy-watch-summary",
		prop("cursor", "pointer"),
		prop("color", "var(--text-dim)"),
		prop("font-size", "var(--type-13)"),
		prop("padding", "0.35rem 0"),
	)
}

// registerRhythmBreakpoints stacks the surface at the pane-based thresholds:
// at 966 the hero drops its stat rail beneath the band and the agenda/roster
// grids relax; at 710 everything single-columns and rows compress.
func registerRhythmBreakpoints() {
	ruleContentMax(contentGrid4, ".rhy-hero",
		prop("grid-template-columns", "1fr"),
	)
	ruleContentMax(contentGrid4, ".rhy-stats",
		prop("flex-direction", "row"),
		prop("flex-wrap", "wrap"),
		prop("gap", "1.25rem"),
		prop("padding-left", "0"),
		prop("padding-top", "0.9rem"),
		prop("border-left", "0"),
		prop("border-top", "1px solid var(--border)"),
	)
	ruleContentMax(contentGrid1, ".rhy-claim",
		prop("grid-template-columns", "5rem minmax(0, 1fr) auto"),
		prop("gap", "0.5rem"),
	)
	// Below the single-column threshold the spine's bar is noise — keep the
	// percent, drop the track to reclaim width.
	ruleContentMax(contentGrid1, ".rhy-spine-track",
		prop("display", "none"),
	)
	ruleContentMax(contentGrid1, ".rhy-ag-row",
		prop("grid-template-columns", "3.6rem minmax(0, 1fr) auto"),
		prop("gap", "0.4rem"),
	)
	// The trailing verb wraps under on the tightest widths rather than crushing
	// the name column.
	ruleContentMax(contentGrid1, ".rhy-ag-verb",
		prop("grid-column", "2 / -1"),
		prop("justify-content", "flex-start"),
	)
}
