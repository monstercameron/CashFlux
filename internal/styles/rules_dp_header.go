// SPDX-License-Identifier: MIT

package styles

// registerDpHeader is the 2026-07-19 "quiet the global top header" pass from the
// frontend-design review. The top bar had grown a row of equally-weighted icon
// controls (music, activity/history, Smart-insights peek, notifications, + Add,
// the add-anything caret, and ⋯ More), so nothing read as dominant. The refinement
// relocates the low-frequency ambient controls — music, the activity/history
// "Updated …" stamp, and the Smart peek — into the existing ⋯ More overflow, leaving
// the actions row to just the two dominant controls (notifications + the primary
// + Add). This file styles the small cluster those relocated controls form at the
// top of the More popover so it reads as an intentional quick-controls strip rather
// than three loose icons.
//
// Layout/tokens only (theme tokens, so light and dark track automatically);
// registered LAST in install.go so it wins at equal specificity.
func registerDpHeader() {
	// The relocated ambient controls sit in a calm row at the top of the ⋯ More
	// popover, separated from the labeled action rows below by a hairline divider.
	rule(".add-menu .tb-more-quick",
		display("flex"),
		alignItems("center"),
		flexWrap("wrap"),
		gap("0.15rem"),
		paddingBottom("0.35rem"),
		marginBottom("0.25rem"),
		borderBottom("1px solid var(--border)"),
	)
	// The music toggle is force-hidden in the bar below 1280px by rules_headerbalance;
	// inside the More popover it must always be visible when the menu is open, so the
	// user can reach it at every width. Scope the override to the cluster only.
	rule(".add-menu .tb-more-quick .muzak-btn",
		display("inline-flex !important"),
	)
	// The relocated controls carry top-bar sizing/hover intended for the actions row.
	// Give them a consistent menu-row footprint so the cluster reads as one strip.
	rule(".add-menu .tb-more-quick .muzak-btn, .add-menu .tb-more-quick .smart-peek-tb, .add-menu .tb-more-quick .tb-updated",
		padding("0.3rem 0.4rem"),
	)
	// The activity/history "Updated …" stamp collapses to icon-only in the crowded
	// top bar (rules_dp_header's max-width:1720px rule hides .tb-updated-label). But
	// inside the ⋯ More popover that same viewport-width media query still matched,
	// so the relocated stamp showed as a lone clock glyph with no label — an unlabeled
	// empty row under "ACTIVITY & INSIGHTS" (task #4). Force the label visible in the
	// menu (higher specificity beats the media rule; media queries add no specificity).
	rule(".add-menu .tb-more-quick .tb-updated .tb-updated-label",
		display("inline"),
	)
}
