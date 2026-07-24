// SPDX-License-Identifier: MIT

package styles

// registerSyncPulse styles the top-bar liveness indicator (SyncPulse): a small
// cloud that sits quiet and dim and flashes once per real sync.
//
// The whole design constraint is "reassuring without being noticeable". At rest
// it has no motion at all and reads as chrome; a sync produces one 450ms
// brighten-and-settle — the motion spec's ceiling for routine motion, and the
// longest the scale allows, because this is a glanceable confirmation rather
// than feedback on something the user just clicked. It never moves the layout:
// the flash animates colour and opacity only, so a sync mid-sentence cannot
// nudge anything the eye is tracking.
func registerSyncPulse() {
	// The slot is always present so the indicator keeps its place in the bar when it
	// appears (see SyncPulse's comment on late-mount append).
	rule(".sync-pulse-slot",
		display("inline-flex"),
		alignItems("center"),
	)
	rule(".sync-pulse",
		display("inline-flex"),
		alignItems("center"),
		gap("0.25rem"),
		padding("0.25rem"),
		border("0"),
		background("transparent"),
		color("var(--text-faint)"),
		cursor("pointer"),
		borderRadius("var(--radius-sm, 6px)"),
		// Colour only — never size or position. A status indicator that reflows the
		// top bar makes every sync a small distraction.
		transition("color var(--motion-fast) var(--ease-standard), opacity var(--motion-fast) var(--ease-standard)"),
	)
	rule(".sync-pulse:hover",
		color("var(--text)"),
	)
	// The tones mirror the rail chip's palette so one glance at either surface
	// means the same thing. At rest every tone is dimmed to chrome weight; the
	// colour only asserts itself on a flash or a problem state.
	rule(".sync-pulse.sync-ok",
		color("var(--text-faint)"),
	)
	rule(".sync-pulse.sync-off",
		color("var(--text-faint)"),
		opacity("0.55"),
	)
	rule(".sync-pulse.sync-warn",
		color("var(--warn, #f59e0b)"),
	)
	rule(".sync-pulse.sync-err",
		color("var(--danger)"),
	)
	// One shot per sync. Latched to 450ms in Go (a push completes far too fast to
	// see), so this runs exactly once and settles — no bounce, no second beat.
	rule(".sync-pulse.sync-pulse-flash",
		animation("cf-sync-pulse var(--motion-narrative) var(--ease-exit) 1"),
	)
	keyframes("cf-sync-pulse",
		at("0%",
			color("var(--accent)"),
			opacity("1"),
		),
		at("100%",
			color("var(--text-faint)"),
			opacity("1"),
		),
	)
	// A queue that is NOT draining is the one thing worth showing continuously: it
	// means work is outstanding, not that work is happening. Reuses the chip's
	// existing pulse loop rather than inventing a second idle animation.
	rule(".sync-pulse.sync-pulse-busy",
		animation("cf-pulse 1s ease-in-out infinite"),
	)
	rule(".sync-pulse .sync-pulse-count",
		fontSize("var(--type-11, 11px)"),
		fontVariantNumeric("tabular-nums"),
		opacity("0.8"),
	)
	// Reduced motion: the indicator still CHANGES on a sync — losing the signal
	// entirely would defeat the point — it just changes without animating. The
	// flash becomes a static accent tint for the same latched window.
	ruleMedia("(prefers-reduced-motion: reduce)", ".sync-pulse.sync-pulse-flash",
		animation("none"),
		color("var(--accent)"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".sync-pulse.sync-pulse-busy",
		animation("none"),
	)
}
