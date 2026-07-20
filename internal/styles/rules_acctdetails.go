// SPDX-License-Identifier: MIT

package styles

// registerAcctDetails structures the per-account "Details" disclosure panel
// (accounts_row.go detailsNode). Before this the revealed extras — sparkline,
// documents drawer, set-institution nudge — floated as bare unaligned fragments
// and the panel read as unfinished scaffolding (UI/UX review task #7). The
// panel now reads as a quiet sub-section: separated from the row head by a
// subtle rule, items stacked on one left edge with even rhythm, and the 90-day
// sparkline presented as a captioned figure. Registered from Register().
func registerAcctDetails() {
	rule(".acct-row-details",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("align-items", "flex-start"),
		prop("gap", "0.45rem"),
		prop("margin-top", "0.5rem"),
		prop("padding-top", "0.55rem"),
		prop("border-top", "1px solid var(--border-subtle)"),
		prop("min-width", "0"),
	)
	// The 90-day balance figure: line + caption on one baseline-aligned row.
	rule(".acct-spark-fig",
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("gap", "0.6rem"),
	)
	rule(".acct-spark-fig .acct-spark",
		prop("display", "block"),
		prop("flex-shrink", "0"),
	)
	// The missing-institution nudge is an action, not stray text.
	rule(".acct-set-institution",
		prop("align-self", "flex-start"),
		prop("text-align", "left"),
		prop("font-size", "var(--type-13)"),
	)
}
