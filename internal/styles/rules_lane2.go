// SPDX-License-Identifier: MIT

package styles

// registerLane2Dashboard styles the 2026-07-17 dashboard-defaults pass (#76):
// the focused-mode hero compaction, the calm-by-default bento (rearranging
// chrome only in edit-layout mode), the Daily check-in recommendation strip,
// the bills "View all" row, and the Needs-attention money/household grouping.
// Registered after registerGenerated() so equal-specificity refinements win.
func registerLane2Dashboard() {
	// --- Focused-mode hero: ~30% shorter while a curated Focus view is active.
	// The sparkline is the hero's tallest optional block; a focus view is about
	// the day's few decisions, not the six-month trend (it stays on the trend
	// widget and the full view).
	rule(".home-hero--focused .home-hero-spark",
		display("none"),
	)
	rule(".home-hero--focused",
		paddingTop("0.9rem"),
		paddingBottom("0.9rem"),
	)
	rule(".home-hero--focused .home-hero-top",
		marginBottom("0.25rem"),
	)
	rule(".home-hero--focused .home-hero-greeting",
		fontSize("1.15rem"),
	)
	rule(".home-hero--focused .home-hero-nw-fig",
		fontSize("1.9rem"),
	)
	rule(".home-hero--focused .home-hero-main",
		marginBottom("0.35rem"),
	)
	rule(".home-hero--focused .home-hero-stats",
		marginTop("0.35rem"),
		marginBottom("0.35rem"),
	)

	// --- Calm-by-default bento (#76): outside explicit edit-layout mode the
	// drag grips and resize handles are gone, not merely dimmed. (Pointer drag
	// is disabled attribute-side in the widget shell; keyboard grab/move stays.)
	rule(`.bento[data-layout-edit="off"] .w .grip`,
		display("none"),
	)
	rule(`.bento[data-layout-edit="off"] .rz`,
		display("none"),
	)
	// In edit mode the grips read as always-present affordances, not hover chrome.
	rule(`.bento[data-layout-edit="on"] .w`,
		borderStyle("dashed"),
	)

	// --- Daily check-in recommendation strip: one quiet line under the hero
	// actions, reading as a suggestion rather than a banner.
	rule(".dash-daily-nudge",
		display("flex"),
		alignItems("center"),
		gap("0.6rem"),
		flexWrap("wrap"),
		marginTop("0.6rem"),
		padding("0.5rem 0.75rem"),
		border("1px dashed var(--border)"),
		borderRadius("10px"),
		background("var(--bg-elev)"),
	)
	rule(".dash-daily-nudge-text",
		fontSize("0.85rem"),
		color("var(--text-dim)"),
	)

	// --- Bills glance: the trailing "View all N bills" row.
	rule(".dash-view-all",
		display("inline-flex"),
		alignItems("center"),
		gap("0.3rem"),
		marginTop("0.35rem"),
		padding("0.15rem 0"),
		border("none"),
		background("transparent"),
		color("var(--accent)"),
		fontSize("0.82rem"),
		cursor("pointer"),
	)
	rule(".dash-view-all:hover",
		textDecoration("underline"),
	)

	// --- Needs attention: money vs household grouping.
	rule(".attention-groups",
		display("flex"),
		flexDirection("column"),
		gap("0.5rem"),
	)
	rule(".attn-group-label",
		display("block"),
		fontSize("0.62rem"),
		fontWeight("700"),
		letterSpacing("0.1em"),
		textTransform("uppercase"),
		color("var(--text-faint)"),
		marginBottom("0.25rem"),
	)
}
