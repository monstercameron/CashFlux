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
		warn  = "#d98c00" // the app's attention amber, for a behind-pace pace rail
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

	// Heading row: the label on the left, a status pill (ahead / on pace / behind) on the
	// right — the pill is the at-a-glance verdict.
	rule(".gtj-head2",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.5rem"),
	)
	rule(".gtj-pill",
		display("inline-flex"),
		alignItems("center"),
		flexShrink("0"),
		padding("0.08rem 0.5rem"),
		borderRadius("999px"),
		border("1px solid var(--border)"),
		fontSize("0.7rem"),
		fontWeight("600"),
		whiteSpace("nowrap"),
		color("var(--text-dim)"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".gtj-pill.is-ahead, .gtj-pill.is-done",
		color("var(--accent)"),
		borderColor("color-mix(in srgb, var(--accent) 45%, var(--border))"),
		background("color-mix(in srgb, var(--accent) 12%, transparent)"),
	)
	rule(".gtj-pill.is-behind",
		color(warn),
		borderColor("color-mix(in srgb, "+warn+" 45%, var(--border))"),
		background("color-mix(in srgb, "+warn+" 12%, transparent)"),
	)
	rule(".gtj-pill.is-neutral",
		color("var(--text)"),
	)

	// The pace rail: a thin timeline from "now" (left) to the horizon (right). A fill runs
	// to the month the goal is reached; a flag dot marks that point; a neutral tick marks
	// the target date — so a flag LEFT of the tick reads as "ahead", right of it "behind".
	rule(".gtj-rail",
		position("relative"),
		height("8px"),
		marginTop("0.55rem"),
		borderRadius("999px"),
		background("color-mix(in srgb, var(--text) 9%, transparent)"),
	)
	rule(".gtj-rail-fill",
		position("absolute"),
		left("0"),
		top("0"),
		bottom("0"),
		borderRadius("999px"),
		minWidth("6px"),
	)
	rule(".gtj-rail-fill.is-ahead, .gtj-rail-fill.is-done",
		background("linear-gradient(90deg, color-mix(in srgb, var(--accent) 45%, transparent), var(--accent))"),
	)
	rule(".gtj-rail-fill.is-behind",
		background("linear-gradient(90deg, color-mix(in srgb, "+warn+" 40%, transparent), "+warn+")"),
	)
	// Flag dot at the projected-completion point — a filled dot ringed in the card colour
	// so it reads clearly over the fill.
	rule(".gtj-rail-flag",
		position("absolute"),
		top("50%"),
		width("12px"),
		height("12px"),
		borderRadius("50%"),
		transform("translate(-50%, -50%)"),
		border("2px solid var(--bg-card)"),
		boxShadow("0 0 0 1px color-mix(in srgb, var(--text) 20%, transparent)"),
	)
	rule(".gtj-rail-flag.is-ahead, .gtj-rail-flag.is-done",
		background("var(--accent)"),
	)
	rule(".gtj-rail-flag.is-behind",
		background(warn),
	)
	// Target-date tick — a slim neutral upright, distinct from the coloured flag dot.
	rule(".gtj-rail-target",
		position("absolute"),
		top("-3px"),
		bottom("-3px"),
		width("2px"),
		transform("translateX(-50%)"),
		borderRadius("1px"),
		background("color-mix(in srgb, var(--text) 55%, transparent)"),
	)
	// Rail end captions: "Now · $X" on the left, "$target · <month>" on the right.
	rule(".gtj-rail-ends",
		display("flex"),
		justifyContent("space-between"),
		gap("0.75rem"),
		marginTop("0.5rem"),
		fontSize("0.72rem"),
		fontVariantNumeric("tabular-nums"),
		color("var(--text-dim)"),
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
