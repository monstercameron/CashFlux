// SPDX-License-Identifier: MIT

package styles

// registerNotifySurface restyles the Notifications header after its redesign: the old
// count tile and the near-empty filter strip are merged into ONE header card whose
// severity chips ARE the filter. Registered after registerGenerated() so these rules
// win the cascade over the original .notif-summary / .notif-sev-chip declarations.
func registerNotifySurface() {
	// Tighter row rhythm. The feed rows were ~90-105px tall for a title + one line of
	// body + a "severity · time" foot, so only ~6 of 13 alerts fit on screen. Trim the
	// vertical padding, the body line-gap, the medallion, and the inter-row gap so the
	// same triage log shows ~9-10 alerts at once without feeling cramped.
	rule(".notif",
		padding("0.55rem 0.8rem"),
	)
	rule(".notif-body",
		gap("0.15rem"),
	)
	rule(".notif-badge",
		width("30px"),
		height("30px"),
	)
	rule(".notif-list",
		gap("0.4rem"),
	)

	// Row actions were three equally-bold BORDERED buttons on every single row — ~24
	// identical square glyphs marching down the right edge, reading as button-soup with
	// the same visual weight as the alert content. Quiet them at rest (borderless,
	// dimmed) and bring them to full contrast when the row is hovered or an action is
	// keyboard-focused; keep them fully visible on touch devices (no hover) so they stay
	// reachable. The alert text now clearly out-weights its controls.
	rule(".notif-icon-btn",
		borderColor("transparent"),
		color("var(--text-faint)"),
		opacity("0.7"),
	)
	rule(".notif:hover .notif-icon-btn, .notif-icon-btn:focus-visible",
		borderColor("var(--text-dim)"),
		color("var(--text)"),
		opacity("1"),
	)
	ruleMedia("(hover: none)", ".notif-icon-btn",
		borderColor("var(--text-dim)"),
		color("var(--text)"),
		opacity("1"),
	)

	// The Live/History toggle sat with a 1.1rem gap below it, floating away from the
	// summary header it belongs to. Snug it up so the switcher reads as part of the
	// notifications surface, not an orphaned control.
	rule(".nhx-head",
		marginBottom("0.65rem"),
	)

	// The right-hand cluster: the filter chips plus the destructive Clear all, pushed
	// to the trailing edge and allowed to wrap under the count on narrow widths.
	rule(".notif-summary-actions",
		display("flex"),
		alignItems("center"),
		gap("0.75rem"),
		marginLeft("auto"),
		flexWrap("wrap"),
		justifyContent("flex-end"),
	)
	rule(".notif-summary-filters",
		display("flex"),
		alignItems("center"),
		gap("0.4rem"),
		flexWrap("wrap"),
	)

	// Chips are now pressable toggles, not static tallies. Reset the button chrome,
	// add affordance (pointer + hover lift), and keep the tabular count legible.
	rule(".notif-sev-chip",
		appearance("none"),
		fontFamily("inherit"),
		cursor("pointer"),
		background("var(--bg)"),
		border("1px solid var(--border)"),
		transition("background .12s ease, border-color .12s ease, color .12s ease, box-shadow .12s ease"),
	)
	rule(".notif-sev-chip:hover",
		borderColor("var(--text-dim)"),
	)
	rule(".notif-sev-chip:focus-visible",
		outline("2px solid var(--accent)"),
		outlineOffset("2px"),
	)
	// The active chip reads as selected — a tinted fill keyed to its severity, so the
	// header doubles as the current-filter indicator.
	rule(".notif-sev-chip.is-active",
		color("var(--text)"),
		borderColor("color-mix(in srgb, var(--accent) 55%, var(--border))"),
		background("color-mix(in srgb, var(--accent) 14%, transparent)"),
	)
	rule(".notif-sev-chip.sev-critical.is-active",
		borderColor("color-mix(in srgb, #ef4444 55%, var(--border))"),
		background("color-mix(in srgb, #ef4444 15%, transparent)"),
	)
	rule(".notif-sev-chip.sev-warning.is-active",
		borderColor("color-mix(in srgb, #f59e0b 55%, var(--border))"),
		background("color-mix(in srgb, #f59e0b 15%, transparent)"),
	)
	// The "All" reset chip has no dot; give its label the same weight as a count so it
	// sits level with the tallied chips beside it.
	rule(".notif-sev-chip.all .notif-sev-name",
		color("var(--text)"),
		fontWeight("600"),
	)

	// Clear all: a quiet ghost button until hovered, when it turns destructive-red
	// (the existing .notif-clear:hover rule). It used to borrow .strip-toggle chrome;
	// now it owns its base style.
	rule(".notif-clear",
		appearance("none"),
		fontFamily("inherit"),
		cursor("pointer"),
		display("inline-flex"),
		alignItems("center"),
		padding("0.3rem 0.7rem"),
		borderRadius("999px"),
		border("1px solid var(--border)"),
		background("transparent"),
		color("var(--text-dim)"),
		fontSize("0.8rem"),
		transition("border-color .12s ease, color .12s ease"),
	)

	// --- collapsed groups (task: friendly, never naggy) ------------------------
	// A run of same-kind, non-critical alerts (e.g. eight "needs a balance update")
	// collapses into ONE summary card. The card carries the same severity accent as
	// a single row so it reads as part of the same triage log, plus a disclosure
	// that expands to the individual rows.
	rule(".notif-group",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		flexWrap("wrap"),
		padding("0.55rem 0.8rem"),
		borderRadius("var(--radius)"),
		border("1px solid var(--border)"),
		background("var(--bg-card)"),
	)
	rule(".notif-group.sev-warning",
		borderColor("color-mix(in srgb, #f59e0b 45%, var(--border))"),
	)
	rule(".notif-group.sev-info",
		borderColor("var(--border)"),
	)
	// The head is the whole clickable summary bar; the toggle button fills it and the
	// dismiss-all sits at the trailing edge.
	rule(".notif-group-head",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		width("100%"),
	)
	rule(".notif-group-toggle",
		appearance("none"),
		fontFamily("inherit"),
		cursor("pointer"),
		display("flex"),
		alignItems("center"),
		gap("0.6rem"),
		flex("1 1 auto"),
		minWidth("0"),
		padding("0"),
		border("none"),
		background("transparent"),
		color("var(--text)"),
		textAlign("left"),
	)
	rule(".notif-group-body-text",
		display("flex"),
		flexDirection("column"),
		gap("0.05rem"),
		minWidth("0"),
	)
	rule(".notif-group-summary",
		fontWeight("600"),
		fontSize("0.9rem"),
		color("var(--text)"),
	)
	rule(".notif-group-hint",
		fontSize("0.75rem"),
		color("var(--text-faint)"),
	)
	// The disclosure pill: "Show all ⌄" / "Hide ⌄", chevron rotates when open.
	rule(".notif-group-disc",
		display("inline-flex"),
		alignItems("center"),
		gap("0.2rem"),
		marginLeft("auto"),
		fontSize("0.8rem"),
		color("var(--text-dim)"),
		whiteSpace("nowrap"),
	)
	rule(".notif-group-disc svg",
		transition("transform .15s ease"),
	)
	rule(".notif-group.is-open .notif-group-disc svg",
		transform("rotate(180deg)"),
	)
	// The expanded child rows sit indented under the summary, spanning the full width.
	rule(".notif-group-list",
		display("flex"),
		flexDirection("column"),
		gap("0.4rem"),
		width("100%"),
		marginTop("0.5rem"),
		paddingTop("0.5rem"),
		borderTop("1px solid var(--border)"),
	)
}
