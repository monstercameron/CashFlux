// SPDX-License-Identifier: MIT

package styles

// registerDashTodo applies the 2026-07-19 Dashboard + To-do UX-polish pass. It is
// registered LAST in install.go (after registerDashHeroSurface and registerDtxPolish),
// so these equal-specificity rules win the cascade over the earlier hero overrides
// without editing rules_gen.go or the shared surface files. All colour comes from
// theme tokens (--accent / --text / --text-dim / --border), so every rule reads
// correctly in both light and dark themes.
//
// Three refinements:
//   - Dashboard hero: shave another ~25-30% of height off the net-worth band so the
//     "Needs attention" digest and the operational action cards clear the first fold
//     even sooner (builds on the two earlier hero shrinks — does not undo them).
//   - Dashboard widgets: a light primary/secondary tone so the action cards read as
//     more important than the recap cards, instead of every tile competing equally.
//   - To-do: an expandable affordance on clamped task notes.
func registerDashTodo() {
	registerDashHeroShrink()
	registerDashWidgetTone()
	registerTodoNoteExpand()
}

// registerDashHeroShrink trims the net-worth hero further. Each earlier pass
// (registerDashHeroSurface, then registerDtxPolish) already tightened it; this one
// takes the padding, the headline figure, the sparkline, and the inter-section gaps
// down another notch so the actionable content below the hero rises into view.
func registerDashHeroShrink() {
	// Less bordered padding all round, and a tighter gap to the bento below.
	rule(".home-hero",
		padding("0.85rem 1.7rem 0.8rem"),
		marginBottom("0.7rem"),
	)
	// The greeting is warmth, not the headline — a compound selector so this beats the
	// utility font-size folded onto the H2 (single-class utilities otherwise win).
	rule(".home-hero .home-hero-greeting",
		fontSize("var(--type-20)"),
	)
	rule(".home-hero-top",
		marginBottom("0.3rem"),
	)
	// The net-worth figure stays the anchor but steps down again (2.5rem → ~2.05rem):
	// the KPI row directly below already carries the number too.
	rule(".home-hero-nw-fig",
		fontSize("2.05rem"),
	)
	// A living sparkline, not a hero-height chart — trim it from 54px to 40px.
	rule(".home-hero-spark svg",
		height("40px"),
	)
	// Pull the two stacked sections below the headline up.
	rule(".home-hero-main",
		gap("1.9rem"),
	)
	rule(".home-hero-stats",
		marginTop("0.5rem"),
		paddingTop("0.45rem"),
	)
	rule(".home-hero-actions",
		marginTop("0.5rem"),
	)
}

// registerDashWidgetTone gives the dashboard's action-oriented tiles a subtle primary
// tone (a faint accent surface wash + a 2px accent top edge) and recedes the recap
// tiles' headings, so the two groups no longer compete equally. It's a tone layer, not
// a redesign: the shared .w card shell, its sizing, and its hover behaviour are
// untouched; these rules only add the accent wash / dim the recap headings. Targeted by
// the stable data-widget id the Widget shell already emits.
func registerDashWidgetTone() {
	// PRIMARY (action) tiles — the ones the user acts ON: attention digest, freshness
	// nudges, the to-do list, the cash forecast, and the smart/anomaly hubs. A 5% accent
	// fill plus a 2px accent top bar (the bar is listed first so it paints over the fill).
	primary := "" +
		".w[data-widget=\"attention\"]," +
		".w[data-widget=\"freshness\"]," +
		".w[data-widget=\"todo\"]," +
		".w[data-widget=\"forecast\"]," +
		".w[data-widget=\"smart-digest\"]," +
		".w[data-widget=\"anomaly-hub\"]"
	rule(primary,
		boxShadow("inset 0 2px 0 0 color-mix(in srgb, var(--accent) 55%, transparent), inset 0 0 0 100px color-mix(in srgb, var(--accent) 5%, transparent)"),
	)

	// SECONDARY (recap) tiles — the ones the user reads: recent activity, the net-worth
	// trend, the spending breakdown, cash-flow, bills, monthly recap, the highlight. Dim
	// their heading so they recede a half-step beneath the action cards.
	secondaryHead := "" +
		".w[data-widget=\"recent\"] .wh h2," +
		".w[data-widget=\"trend\"] .wh h2," +
		".w[data-widget=\"breakdown\"] .wh h2," +
		".w[data-widget=\"cashflow\"] .wh h2," +
		".w[data-widget=\"bills\"] .wh h2," +
		".w[data-widget=\"monthly-recap\"] .wh h2," +
		".w[data-widget=\"highlight\"] .wh h2"
	rule(secondaryHead,
		color("var(--text-dim)"),
		fontWeight("500"),
	)
}

// registerTodoNoteExpand turns a clamped task note into a clear, easy expand affordance.
// The base .todo-meta-note (rules_gen.go) stays a single-line ellipsis; a note long
// enough to be clamped gets the .is-expandable modifier, which adds a resting dotted
// underline cue and, on hover or keyboard focus, unclamps to wrap the full note text.
func registerTodoNoteExpand() {
	// Resting affordance: a quiet dotted underline + pointer so a clamped note reads as
	// "there's more here".
	rule(".todo-meta-note.is-expandable",
		cursor("pointer"),
		prop("border-bottom", "1px dotted color-mix(in srgb, var(--text) 30%, transparent)"),
	)
	// Expanded: drop the single-line clamp, let the note wrap, and brighten it from the
	// faint resting tone to full foreground so it's comfortable to read.
	rule(".todo-meta-note.is-expandable:hover, .todo-meta-note.is-expandable:focus-visible",
		prop("white-space", "normal"),
		prop("overflow", "visible"),
		maxWidth("40rem"),
		lineHeight("1.4"),
		color("var(--text)"),
		prop("text-overflow", "clip"),
	)

	// The "N of M done" count under the To-do completion percentage: small, dim, and
	// tabular so the digits line up beneath the big percent.
	rule(".todo-done-count",
		fontSize("var(--type-12)"),
		color("var(--text-dim)"),
		prop("font-variant-numeric", "tabular-nums"),
		prop("margin-top", "0.1rem"),
	)
}
