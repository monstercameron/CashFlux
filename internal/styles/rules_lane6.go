// SPDX-License-Identifier: MIT

package styles

// registerLane6Fixes holds the 2026-07-17 lane-6 remediation rules.
func registerLane6Fixes() {
	// UX-08: the flip modal's front face is decorative (aria-hidden) and rotates
	// away behind the back face, but backface-visibility alone leaves its H3 in
	// hit-testing/visibility terms "visible" — a residual second dialog title.
	// Once the flip completes (450ms — the v1.2.3 narrative token cap), the front
	// face becomes visibility:hidden (delayed so the animation itself is
	// untouched); reduced-motion skips the delay along with the flip.
	rule(".flip-inner.flipped .flip-face:not(.flip-back)",
		prop("visibility", "hidden"),
		transition("visibility 0s var(--motion-narrative, 450ms)"),
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
	ruleContentMax(contentTwoCol, ".ask-aside-toggle",
		display("inline-flex"),
	)
	// Narrow widths: the aside becomes a right-hand slide-in drawer (it used to
	// stack below the chat, burying saved conversations under the whole thread).
	ruleContentMax(contentTwoCol, ".ask-aside",
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
	ruleContentMax(contentTwoCol, ".ask-aside.is-open",
		transform("translateX(0)"),
	)
	ruleContentMax(contentTwoCol, ".ask-aside-close",
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
	ruleContentMax(contentTwoCol, ".ask-aside-backdrop",
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

	// ── #67: contrast repairs the token fix can't reach ────────────────────────
	// The smart-peek badge's red (#d8716f) lands ~3.45:1 on its own translucent
	// red fill; pull the glyph color toward the text so it clears AA while the
	// badge keeps its red identity from the fill + border.
	// Double-class selector: the badge's tone arrives via an atomic color class
	// (.text-down + generated class) that outlasts a bare .smart-peek-badge rule.
	rule(".smart-peek-badge.text-down, span.smart-peek-badge",
		color("color-mix(in srgb, #d8716f 45%, var(--text))"),
	)
	ruleMedia("(prefers-color-scheme: light)", "[data-theme=\"light\"] .smart-peek-badge",
		color("color-mix(in srgb, #d8716f 55%, #000)"),
	)
	rule("[data-theme=\"light\"] .smart-peek-badge",
		color("color-mix(in srgb, #d8716f 55%, #000)"),
	)
	// The raw accent (#2e8b57) as SMALL TEXT is ~4.4:1 on dark surfaces — just
	// under AA. These text usages switch to the theme-derived --accent-ink
	// (accent pulled toward the text color until it passes); fills keep --accent.
	rule(".th-sort",
		color("var(--accent-ink, var(--accent))"),
	)
	// Match the original sorted-column selector's specificity so the ink wins.
	rule(".txn-table th[aria-sort=\"ascending\"] .th-sort, .txn-table th[aria-sort=\"descending\"] .th-sort",
		color("var(--accent-ink, var(--accent))"),
	)
	// The todo List/Board view switch's active pill: accent text on accent tint.
	rule(".tvw-btn.is-active",
		color("var(--accent-ink, var(--accent))"),
	)
	rule(".txn-followup-chip span",
		color("var(--accent-ink, var(--accent))"),
	)
	rule(".nhx-toggle-btn[aria-selected=\"true\"]",
		color("var(--accent-ink, var(--accent))"),
	)
	rule(".notif-sev-tag.sev-info",
		color("var(--accent-ink, var(--accent))"),
	)
	rule(".btn-link",
		color("var(--accent-ink, var(--accent))"),
	)
	// Active goals/board tab: #04140c on the mid-green accent was 4.4:1, and
	// white on the raw accent is 4.27:1 — the mid-tone fails against both. A
	// slightly darkened fill with white text clears AA in both themes.
	rule(".goals-tab.is-active",
		background("color-mix(in srgb, var(--accent) 78%, #000)"),
		color("#ffffff"),
	)
	rule("[data-theme=\"light\"] .goals-tab.is-active",
		background("color-mix(in srgb, var(--accent) 78%, #000)"),
		color("#ffffff"),
	)
	// White 12.5px text on the raw danger red (#d8716f) was 3.27:1; darken the
	// fill, keep white text (≈5.9:1).
	rule(".budget-rail-resolve",
		background("color-mix(in srgb, var(--danger, #d8716f) 62%, #000)"),
	)
	// .set-label hard-coded #6c6c72 (3.8:1); use the AA-derived faint token.
	rule(".set-label",
		color("var(--text-faint)"),
	)
	// The goal card's inline target button renders OVER the loader's accent
	// progress fill once coverage reaches it — gray text on green (1.86:1). A
	// near-opaque card-toned backdrop restores a known background.
	rule(".goal-card-loader .budget-limit-btn",
		background("color-mix(in srgb, var(--bg-card) 88%, transparent)"),
		borderRadius("6px"),
		padding("0 0.3rem"),
	)
	// Annual-report sparkline captions render over the chart's gradient wash;
	// lift them to the dim tone so they clear AA over the lightest band.
	rule(".rpta-fig-spark-cap, .rpta-fig-spark-cap .rpta-src",
		color("var(--text-dim)"),
	)
}
