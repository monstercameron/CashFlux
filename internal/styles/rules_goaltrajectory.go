// SPDX-License-Identifier: MIT

package styles

// registerGoalTrajectorySurface emits the per-goal savings-trajectory chart
// section (`.gtj-*`) that sits inside each goal card below the progress meter: a
// small heading, the reused AreaChart, and a one-line ETA readout. It mirrors the
// calm, hairline-bordered look of the /goals redesign (see registerGoalsSurface)
// and uses only theme tokens (var(--text)/(--border)/(--bg-card)/(--bg-elev)/
// --accent) — never var(--fg)/(--line)/(--dim)/(--faint) (those are undefined and
// render dark). Registered by the styles coordinator in install.go.
func registerGoalTrajectorySurface() {
	const (
		serif = "var(--font-display), Fraunces, Georgia, serif"
		hair  = "1px solid color-mix(in srgb, var(--border) 60%, transparent)"
	)

	// The section wrapper: a quiet block set off from the figures above with a
	// hairline rule and a little breathing room.
	rule(".gtj",
		display("flex"),
		flexDirection("column"),
		gap("0.4rem"),
		marginTop("0.6rem"),
		paddingTop("0.75rem"),
		borderTop(hair),
	)

	// Heading: a small uppercase label, matching the goal figures' key style so the
	// section reads as part of the same card language.
	rule(".gtj-head",
		fontSize("0.66rem"),
		letterSpacing("0.09em"),
		prop("text-transform", "uppercase"),
		fontWeight("600"),
		color("var(--text-dim)"),
	)

	// Chart holder: constrains the reused AreaChart's height and lets it stretch to
	// the full width of the card's content column.
	rule(".gtj-chart",
		position("relative"),
		width("100%"),
		marginTop("0.15rem"),
	)

	// ETA readout: a plain, readable one-liner. The target month reads in the accent
	// so the "when" pops without shouting.
	rule(".gtj-eta",
		fontSize("0.85rem"),
		lineHeight("1.4"),
		color("var(--text)"),
	)
	rule(".gtj-eta .gtj-eta-when",
		fontFamily(serif),
		fontWeight("600"),
		color("var(--accent)"),
	)

	// Muted / empty state: the low-pressure "add a contribution to see this" note.
	rule(".gtj-eta.is-muted",
		color("var(--text-dim)"),
		fontStyle("italic"),
	)
}
