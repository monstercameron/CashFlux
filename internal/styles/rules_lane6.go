// SPDX-License-Identifier: MIT

package styles

// registerLane6Fixes holds the 2026-07-17 lane-6 remediation rules.
func registerLane6Fixes() {
	// UX-08: the flip modal's front face is decorative (aria-hidden) and rotates
	// away behind the back face, but backface-visibility alone leaves its H3 in
	// hit-testing/visibility terms "visible" — a residual second dialog title.
	// Once the 550ms flip completes, the front face becomes visibility:hidden
	// (delayed so the animation itself is untouched); reduced-motion skips the
	// delay along with the flip.
	rule(".flip-inner.flipped .flip-face:not(.flip-back)",
		prop("visibility", "hidden"),
		transition("visibility 0s .55s"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".flip-inner.flipped .flip-face:not(.flip-back)",
		transition("none"),
	)

	// ── UX-09: assistant restructure ────────────────────────────────────────────
	// Header: identity on the left, the New chat / Chat settings / Notes actions
	// on the right, on one row that wraps gracefully.
	rule(".ask-head-row",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.6rem"),
		width("100%"),
		flexWrap("wrap"),
	)
	rule(".ask-head-actions",
		display("flex"),
		alignItems("center"),
		gap("0.4rem"),
		flexWrap("wrap"),
		// Override the legacy flex:0 0 auto — the cluster must shrink inside the
		// header row so its buttons wrap instead of clipping at phone widths.
		flex("1 1 auto"),
		minWidth("0"),
		justifyContent("flex-end"),
	)
	// The settings drawer is the old control cell, revealed below the header.
	rule(".ask-settings-drawer",
		marginBottom("0.6rem"),
	)
	// Composer-adjacent metadata: privacy badge + scope line left, keyboard hint right.
	rule(".chat-dock-meta",
		display("flex"),
		alignItems("center"),
		gap("0.6rem"),
		flexWrap("wrap"),
		marginTop("0.45rem"),
	)
	rule(".chat-dock-meta .chat-dock-hint",
		margin("0"),
		marginLeft("auto"),
	)
	rule(".chat-privacy-badge",
		appearance("none"),
		fontFamily("inherit"),
		cursor("pointer"),
		display("inline-flex"),
		alignItems("center"),
		gap("0.25rem"),
		padding("0.15rem 0.55rem"),
		borderRadius("999px"),
		border("1px solid var(--border)"),
		background("transparent"),
		color("var(--text-dim)"),
		fontSize("0.7rem"),
		whiteSpace("nowrap"),
	)
	rule(".chat-privacy-badge:hover",
		borderColor("var(--text-dim)"),
		color("var(--text)"),
	)
	rule(".chat-dock-scope",
		fontSize("0.7rem"),
		color("var(--text-faint)"),
	)
	// The aside close button and the header's Notes trigger exist for the narrow
	// drawer only; desktop keeps the always-visible margin-notes column.
	rule(".ask-aside-toggle",
		display("none"),
	)
	rule(".ask-aside-close",
		display("none"),
	)
	rule(".ask-aside-backdrop",
		display("none"),
	)
	ruleMedia("(max-width: 1100px)", ".ask-aside-toggle",
		display("inline-flex"),
	)
	// Narrow widths: the aside becomes a right-hand slide-in drawer (it used to
	// stack below the chat, burying saved conversations under the whole thread).
	ruleMedia("(max-width: 1100px)", ".ask-aside",
		position("fixed"),
		top("0"),
		right("0"),
		bottom("0"),
		width("min(20rem, 86vw)"),
		background("var(--bg-card)"),
		borderLeft("1px solid var(--border)"),
		padding("2.6rem 1rem 1rem"),
		prop("overflow-y", "auto"),
		zIndex("95"),
		transform("translateX(105%)"),
		transition("transform .25s ease"),
		boxShadow("-18px 0 40px -18px rgba(0,0,0,.6)"),
	)
	ruleMedia("(max-width: 1100px)", ".ask-aside.is-open",
		transform("translateX(0)"),
	)
	ruleMedia("(max-width: 1100px)", ".ask-aside-close",
		display("inline-flex"),
		position("absolute"),
		top("0.6rem"),
		right("0.6rem"),
		appearance("none"),
		border("none"),
		background("transparent"),
		color("var(--text-dim)"),
		cursor("pointer"),
		padding("0.3rem"),
	)
	ruleMedia("(max-width: 1100px)", ".ask-aside-backdrop",
		display("block"),
		position("fixed"),
		inset("0"),
		background("rgba(0,0,0,.45)"),
		zIndex("90"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".ask-aside",
		transition("none"),
	)

	// ── #52: detection-confidence chips ────────────────────────────────────────
	// Small labeled tier chips on subscription rows; the WHY lives in the
	// tooltip/aria. Confirmed reads settled (green tint), Likely stays neutral,
	// Needs review carries the amber "look at me" tint.
	rule(".conf-chip",
		display("inline-flex"),
		alignItems("center"),
		padding("0.05rem 0.45rem"),
		borderRadius("999px"),
		border("1px solid var(--border)"),
		fontSize("0.68rem"),
		fontWeight("600"),
		letterSpacing("0.03em"),
		color("var(--text-dim)"),
		whiteSpace("nowrap"),
		cursor("help"),
	)
	rule(".conf-chip.conf-confirmed",
		borderColor("color-mix(in srgb, #22c55e 45%, var(--border))"),
		color("color-mix(in srgb, #22c55e 60%, var(--text))"),
		background("color-mix(in srgb, #22c55e 10%, transparent)"),
	)
	rule(".conf-chip.conf-review",
		borderColor("color-mix(in srgb, #f59e0b 50%, var(--border))"),
		color("color-mix(in srgb, #f59e0b 65%, var(--text))"),
		background("color-mix(in srgb, #f59e0b 10%, transparent)"),
	)

	// ── #68: the shared five-state chip vocabulary ─────────────────────────────
	// One base shape; five state tones. Healthy green / Watch amber / Action red /
	// Blocked neutral / Unconfirmed violet — always paired with the word, so the
	// tone is never the only cue.
	rule(".state-chip",
		display("inline-flex"),
		alignItems("center"),
		padding("0.05rem 0.45rem"),
		borderRadius("999px"),
		border("1px solid var(--border)"),
		fontSize("0.68rem"),
		fontWeight("600"),
		letterSpacing("0.03em"),
		color("var(--text-dim)"),
		whiteSpace("nowrap"),
	)
	rule(".state-chip.state-healthy",
		borderColor("color-mix(in srgb, #22c55e 45%, var(--border))"),
		color("color-mix(in srgb, #22c55e 60%, var(--text))"),
		background("color-mix(in srgb, #22c55e 10%, transparent)"),
	)
	rule(".state-chip.state-watch",
		borderColor("color-mix(in srgb, #f59e0b 50%, var(--border))"),
		color("color-mix(in srgb, #f59e0b 65%, var(--text))"),
		background("color-mix(in srgb, #f59e0b 10%, transparent)"),
	)
	rule(".state-chip.state-action",
		borderColor("color-mix(in srgb, #ef4444 50%, var(--border))"),
		color("color-mix(in srgb, #ef4444 65%, var(--text))"),
		background("color-mix(in srgb, #ef4444 10%, transparent)"),
	)
	rule(".state-chip.state-blocked",
		borderColor("var(--border)"),
		color("var(--text-faint)"),
		background("color-mix(in srgb, var(--text) 5%, transparent)"),
	)
	rule(".state-chip.state-unconfirmed",
		borderColor("color-mix(in srgb, #7c83ff 50%, var(--border))"),
		color("color-mix(in srgb, #7c83ff 65%, var(--text))"),
		background("color-mix(in srgb, #7c83ff 10%, transparent)"),
	)
}
