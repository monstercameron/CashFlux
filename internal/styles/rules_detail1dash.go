// SPDX-License-Identifier: MIT

package styles

// registerDetail1Dash emits the dashboard-side CSS for the 2026-07-19 detail-polish
// lane 1: letting the "Needs attention" titles wrap at narrow pane widths (so the
// distinguishing tail survives the expanded-sidebar squeeze) and the quiet standing
// hint at the top of edit-layout mode. Chained from registerDashTodo() so the shared
// install.go Register() list is untouched by this lane.
func registerDetail1Dash() {
	// Needs-attention rows ellipsize on one line by default (.attention-text is
	// nowrap). At narrow pane widths that clips a long title before the part that
	// distinguishes it (which account / which budget). Below the two-column content
	// threshold — the width an expanded sidebar produces on common laptops — let the
	// title wrap to at most two lines instead, so the tail is legible. Pane-based
	// (ruleContentMax mirrors the rail state), not a viewport query.
	ruleContentMax(contentTwoCol, ".attention-text",
		prop("white-space", "normal"),
		prop("display", "-webkit-box"),
		prop("-webkit-line-clamp", "2"),
		prop("-webkit-box-orient", "vertical"),
	)

	// One quiet standing hint at the top of edit-layout mode. It explains the tile
	// drag/resize affordances (which otherwise carry no single explanation) without
	// competing with the grid — a subtle framed caption, not a banner.
	rule(".dash-edit-hint",
		prop("display", "block"),
		prop("margin", "0 0 0.6rem"),
		prop("padding", "0.4rem 0.7rem"),
		prop("font-size", "var(--type-13)"),
		prop("line-height", "1.4"),
		prop("color", "var(--text-dim)"),
		prop("background", "var(--bg-card)"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "var(--radius-lg)"),
	)
}
