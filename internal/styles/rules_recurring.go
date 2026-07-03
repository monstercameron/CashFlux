// SPDX-License-Identifier: MIT

package styles

// registerRecurringSurface emits the /recurring Scheduled-tab design: the bento
// host, the hero figure, the next-30-days schedule rows (calendar medallions),
// the flow cards (cadence tags + share meters), the detected-charge suggestions,
// and the add/edit modal body. Token-based throughout (var(--accent/--bg-elev/
// --border/…)) so it tracks every theme. Registered from Register() after the
// main sheet (a separate file so the surface owns its rules).
func registerRecurringSurface() {
	rule(".bento.bento-recurring",
		prop("grid-template-rows", "auto"),
		prop("grid-auto-rows", "auto"),
	)
	rule(".bento.bento-recurring > .w",
		prop("height", "auto"),
		prop("min-height", "0"),
		prop("overflow", "visible"),
	)

	// Hero: the net monthly figure beside the in/out/count chips.
	rule(".rec-hero",
		prop("display", "flex"),
		prop("align-items", "flex-end"),
		prop("justify-content", "space-between"),
		prop("flex-wrap", "wrap"),
		prop("gap", "1.25rem"),
	)
	rule(".rec-hero-main",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.3rem"),
		prop("min-width", "14rem"),
	)
	rule(".rec-hero-label",
		prop("font-size", "0.72rem"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.08em"),
	)
	rule(".rec-hero-value",
		prop("font-size", "2.6rem"),
		prop("font-weight", "700"),
		prop("line-height", "1.05"),
		prop("font-variant-numeric", "tabular-nums"),
		prop("text-shadow", "0 0 34px color-mix(in srgb, currentColor 25%, transparent)"),
	)
	rule(".rec-hero-sub",
		prop("margin", "0"),
		prop("font-size", "0.9rem"),
	)

	// Next-30-days schedule.
	rule(".rec-up-meta",
		prop("margin", "0 0 0.6rem"),
		prop("font-size", "0.82rem"),
	)
	rule(".rec-up-list",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.45rem"),
	)
	rule(".rec-up-row",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.8rem"),
		prop("padding", "0.55rem 0.85rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "12px"),
		prop("background", "color-mix(in srgb, var(--bg-elev) 42%, transparent)"),
	)
	rule(".rec-up-row.is-overdue",
		prop("border-color", "color-mix(in srgb, var(--danger) 45%, var(--border))"),
		prop("background", "color-mix(in srgb, var(--danger) 7%, var(--bg-elev))"),
	)
	rule(".rec-up-date",
		prop("flex", "0 0 auto"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("align-items", "center"),
		prop("justify-content", "center"),
		prop("width", "2.6rem"),
		prop("height", "2.6rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "10px"),
		prop("background", "var(--bg-elev)"),
		prop("line-height", "1"),
	)
	rule(".rec-up-day",
		prop("font-size", "1.1rem"),
		prop("font-weight", "700"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	rule(".rec-up-mon",
		prop("font-size", "0.6rem"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.06em"),
		prop("margin-top", "0.1rem"),
	)
	rule(".rec-up-main",
		prop("flex", "1 1 auto"),
		prop("min-width", "0"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.15rem"),
	)
	rule(".rec-up-name",
		prop("font-weight", "600"),
		prop("overflow", "hidden"),
		prop("text-overflow", "ellipsis"),
		prop("white-space", "nowrap"),
	)
	rule(".rec-up-tags",
		prop("display", "flex"),
		prop("flex-wrap", "wrap"),
		prop("gap", "0.3rem"),
	)
	rule(".rec-up-amount",
		prop("flex", "0 0 auto"),
		prop("font-weight", "600"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	rule(".rec-up-more",
		prop("margin", "0.25rem 0 0"),
		prop("font-size", "0.8rem"),
	)

	// Small status tags (autopay / auto-post / overdue).
	rule(".rec-tag",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("padding", "0.05rem 0.45rem"),
		prop("border-radius", "999px"),
		prop("border", "1px solid var(--border)"),
		prop("font-size", "0.64rem"),
		prop("font-weight", "700"),
		prop("text-transform", "uppercase"),
		prop("letter-spacing", "0.05em"),
		prop("color", "var(--text-dim)"),
	)
	rule(".rec-tag-overdue",
		prop("border-color", "color-mix(in srgb, var(--danger) 55%, var(--border))"),
		prop("color", "color-mix(in srgb, var(--danger) 75%, var(--text))"),
		prop("background", "color-mix(in srgb, var(--danger) 10%, transparent)"),
	)

	// Flow cards.
	rule(".rec-flow-list",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.6rem"),
	)
	rule(".rec-flow",
		prop("position", "relative"),
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "1rem"),
		prop("padding", "0.9rem 1.1rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "14px"),
		prop("background", "color-mix(in srgb, var(--bg-elev) 48%, transparent)"),
		prop("transition", "transform 0.16s ease, border-color 0.16s ease"),
	)
	rule(".rec-flow:hover",
		prop("border-color", "color-mix(in srgb, var(--accent) 34%, var(--border))"),
		prop("transform", "translateY(-1px)"),
	)
	rule(".rec-cad-tag",
		prop("flex", "0 0 auto"),
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("justify-content", "center"),
		prop("width", "2.4rem"),
		prop("height", "2.4rem"),
		prop("border-radius", "10px"),
		prop("border", "1px solid color-mix(in srgb, var(--accent) 35%, var(--border))"),
		prop("background", "color-mix(in srgb, var(--accent) 8%, transparent)"),
		prop("color", "color-mix(in srgb, var(--accent) 60%, var(--text))"),
		prop("font-size", "0.72rem"),
		prop("font-weight", "700"),
	)
	rule(".rec-flow-body",
		prop("flex", "1 1 auto"),
		prop("min-width", "0"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.35rem"),
	)
	rule(".rec-flow-head",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("flex-wrap", "wrap"),
		prop("gap", "0.4rem"),
	)
	rule(".rec-flow-name",
		prop("font-weight", "700"),
		prop("font-size", "1rem"),
	)
	rule(".rec-flow-meta",
		prop("font-size", "0.8rem"),
	)
	rule(".rec-flow-share",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.5rem"),
	)
	rule(".rec-flow-share .cf-bar",
		prop("flex", "1 1 auto"),
		prop("max-width", "220px"),
	)
	rule(".rec-flow-share-label",
		prop("font-size", "0.72rem"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	// The flow's formula-identity chip (recurring_<slug>_monthly).
	rule(".rec-flow-var",
		prop("font-family", "ui-monospace, SFMono-Regular, Menlo, monospace"),
		prop("font-size", "0.66rem"),
		prop("padding", "0.05rem 0.4rem"),
		prop("border-radius", "6px"),
		prop("color", "color-mix(in srgb, var(--accent) 55%, var(--text))"),
		prop("background", "color-mix(in srgb, var(--accent) 12%, transparent)"),
	)
	// Bills/subscriptions tabs share the surface: their legacy .row lists read as
	// cards inside the bento (border + radius + elevated tint).
	rule(".rec-cardrows .row",
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "12px"),
		prop("padding", "0.6rem 0.85rem"),
		prop("background", "color-mix(in srgb, var(--bg-elev) 42%, transparent)"),
	)
	rule(".rec-cardrows .row + .row",
		prop("margin-top", "0.45rem"),
	)

	// Bills tab: the list scrolls in place while the calendar stays on screen, so
	// hovering a bill can light up its date without either scrolling away.
	rule(".bills-scroll",
		prop("max-height", "62vh"),
		prop("overflow-y", "auto"),
		prop("padding-right", "0.35rem"),
		prop("overscroll-behavior", "contain"),
	)
	rule(".bills-cal-sticky",
		prop("position", "sticky"),
		prop("top", "0.75rem"),
		prop("align-self", "start"),
	)
	// Give the calendar a real share of the two-column row (it used to sit at its
	// natural width), and scale its grid up so dates read at a glance.
	ruleMedia("(min-width: 1024px)", ".bills-layout > :first-child",
		prop("flex", "1 1 52%"),
		prop("min-width", "0"),
	)
	ruleMedia("(min-width: 1024px)", ".bills-cal-sticky",
		prop("flex", "1 1 48%"),
		prop("min-width", "24rem"),
	)
	rule(".bills-cal-sticky .cal-grid",
		prop("gap", "6px"),
	)
	rule(".bills-cal-sticky .cal-cell",
		prop("min-height", "76px"),
		prop("border-radius", "10px"),
		prop("padding", "7px 9px"),
	)
	rule(".bills-cal-sticky .cal-day",
		prop("font-size", "0.95rem"),
	)
	rule(".bills-cal-sticky .cal-head",
		prop("font-size", "0.72rem"),
		prop("padding", "4px 0"),
	)
	// Hover-linked calendar date: hovering a bill row outlines its due-date cell.
	rule(".cal-cell.cal-hl",
		prop("outline", "2px solid var(--accent)"),
		prop("outline-offset", "-2px"),
		prop("border-radius", "8px"),
		prop("background", "color-mix(in srgb, var(--accent) 14%, transparent)"),
	)
	rule(".cal-cell",
		prop("transition", "background 0.12s ease"),
	)
	// Ghost marker: the inactive bills schedule (raw deadline vs smart plan) —
	// hollow, so it reads as "the other view", never as a live due dot.
	rule(".cal-dot--ghost",
		prop("background", "transparent"),
		prop("border", "1.5px dashed color-mix(in srgb, var(--accent) 60%, var(--border))"),
	)

	// Smart pay schedule tile (bills tab).
	rule(".bills-smart-moves, .bills-smart-suggests",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.4rem"),
		prop("margin-top", "0.5rem"),
	)
	rule(".bills-smart-move",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.6rem"),
		prop("padding", "0.5rem 0.75rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "10px"),
		prop("background", "color-mix(in srgb, var(--bg-elev) 42%, transparent)"),
	)
	rule(".bills-smart-move.is-suggest",
		prop("border-style", "dashed"),
		prop("border-color", "color-mix(in srgb, var(--accent) 40%, var(--border))"),
	)
	rule(".rec-tag-suggest",
		prop("border-color", "color-mix(in srgb, var(--accent) 55%, var(--border))"),
		prop("color", "color-mix(in srgb, var(--accent) 60%, var(--text))"),
	)
	rule(".bills-smart-move-text",
		prop("flex", "1 1 auto"),
		prop("min-width", "0"),
		prop("font-size", "0.9rem"),
	)
	rule(".bills-smart-move-amt",
		prop("flex", "0 0 auto"),
		prop("font-variant-numeric", "tabular-nums"),
		prop("font-weight", "600"),
	)
	rule(".bills-smart-keep",
		prop("max-width", "10rem"),
	)
	rule(".bills-smart-delta",
		prop("font-size", "0.72rem"),
		prop("font-weight", "600"),
		prop("margin-top", "0.15rem"),
	)
	// A payment the plan moved onto this payday: accent-filled with a ring so it
	// cannot be mistaken for an ordinary due-date dot.
	rule(".cal-dot--payahead",
		prop("background", "var(--accent)"),
		prop("box-shadow", "0 0 0 2px color-mix(in srgb, var(--accent) 35%, transparent)"),
	)
	rule(".bills-cal-legend",
		prop("font-size", "0.75rem"),
		prop("margin-top", "0.5rem"),
		prop("line-height", "1.4"),
	)
	rule(".bills-smart-bucket-head",
		prop("margin-top", "0.6rem"),
		prop("font-weight", "600"),
		prop("font-size", "0.82rem"),
		prop("border-bottom", "1px solid var(--border)"),
		prop("padding-bottom", "0.25rem"),
	)
	rule(".bills-smart-modal",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.75rem"),
	)
	rule(".bills-smart-setup",
		prop("display", "grid"),
		prop("grid-template-columns", "minmax(11rem, 14rem) 1fr"),
		prop("gap", "0.75rem"),
		prop("align-items", "end"),
	)
	rule(".rec-flow-figs",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("align-items", "flex-end"),
		prop("gap", "0.2rem"),
		prop("flex", "0 0 auto"),
	)
	rule(".rec-flow-monthly",
		prop("font-size", "1.15rem"),
		prop("font-weight", "700"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	rule(".rec-flow-menu",
		prop("margin-left", "auto"),
		prop("flex", "0 0 auto"),
	)

	// Detected-but-unplanned charges: dashed border = a suggestion, not a record.
	rule(".rec-detected-list",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.45rem"),
	)
	rule(".rec-detected",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.8rem"),
		prop("padding", "0.6rem 0.85rem"),
		prop("border", "1px dashed color-mix(in srgb, var(--accent) 40%, var(--border))"),
		prop("border-radius", "12px"),
		prop("background", "color-mix(in srgb, var(--accent) 4%, transparent)"),
	)
	rule(".rec-detected-main",
		prop("flex", "1 1 auto"),
		prop("min-width", "0"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.1rem"),
	)

	// Ghost date medallion: the second+ row of the same day keeps the layout slot
	// but hides the repeated date.
	rule(".rec-up-date.is-ghost",
		prop("visibility", "hidden"),
	)

	// Post-due gains affordance when it will actually act on something.
	rule(".rec-postdue-hot",
		prop("border-color", "color-mix(in srgb, var(--accent) 55%, var(--border))"),
		prop("color", "color-mix(in srgb, var(--accent) 60%, var(--text))"),
	)

	// Empty state + modal body.
	rule(".rec-empty",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("align-items", "flex-start"),
		prop("gap", "0.6rem"),
		prop("padding", "0.5rem 0"),
	)
	rule(".rec-modal-form .rec-modal-toggles",
		prop("grid-column", "1 / -1"),
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.35rem"),
	)
	rule(".rec-modal-form .rec-modal-wide",
		prop("grid-column", "1 / -1"),
	)
	rule(".rec-toggle-disabled",
		prop("opacity", "0.45"),
		prop("pointer-events", "none"),
	)
	rule(".rec-autopost-hint",
		prop("margin", "-0.15rem 0 0"),
		prop("font-size", "0.78rem"),
	)
}
