// SPDX-License-Identifier: MIT

package styles

// registerDetail1Accounts emits the accounts-side CSS for the 2026-07-19 detail-
// polish lane 1: the subdued stale dot and the one-line stale summary shown when
// more than half the visible accounts are stale (so a wall of amber STALE badges
// stops drowning out the signal). Chained from registerAccountsSurface().
func registerDetail1Accounts() {
	// Subdued replacement for the full STALE badge when most accounts are stale: a
	// small, muted dot beside the name. Neutral (not amber) so a mostly-stale list
	// reads calm, with the summary line carrying the actual call to action.
	rule(".acct-stale-dot",
		prop("display", "inline-block"),
		prop("width", "0.5rem"),
		prop("height", "0.5rem"),
		prop("border-radius", "50%"),
		prop("background", "var(--text-dim)"),
		prop("opacity", "0.6"),
		prop("flex-shrink", "0"),
	)
	// The single summary line leading the list in the collapsed state: a quiet framed
	// caption with the "Mark all updated" action, matching the calm treatment of the
	// other accounts-list captions.
	rule(".acct-stale-summary",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("flex-wrap", "wrap"),
		prop("gap", "0.5rem"),
		prop("margin", "0 0 0.6rem"),
		prop("padding", "0.45rem 0.7rem"),
		prop("font-size", "var(--type-13)"),
		prop("color", "var(--text-dim)"),
		prop("background", "var(--bg-card)"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "var(--radius-lg)"),
	)
}
