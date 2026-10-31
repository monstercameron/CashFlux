// SPDX-License-Identifier: MIT

package styles

// registerAcctDetails lays out the per-account "Details" panel (accounts_row.go detailsNode).
// The panel is a two-column grid that fills the row's width: the period figures run across the
// top, the balance chart and the 90-day projection sit on the left, the reading and the account's
// facts sit on the right, and the documents and the institution share a footer line. Below 760px
// it becomes a single column. Registered from Register().
func registerAcctDetails() {
	rule(".acct-row-details",
		prop("display", "grid"),
		prop("grid-template-columns", "minmax(0, 1.6fr) minmax(0, 1fr)"),
		prop("gap", "1.25rem 2.5rem"),
		prop("align-items", "start"),
		prop("margin-top", "0.75rem"),
		prop("padding-top", "1rem"),
		prop("border-top", "1px solid var(--border-subtle)"),
		prop("min-width", "0"),
	)
	ruleMedia("(max-width: 760px)", ".acct-row-details",
		prop("grid-template-columns", "minmax(0, 1fr)"),
	)
	rule(".acct-row-details > *",
		prop("min-width", "0"),
		prop("max-width", "100%"),
	)

	// The period figures: three equal columns across the top, each a quiet label over the amount.
	rule(".acct-flow",
		prop("grid-column", "1 / -1"),
		prop("display", "grid"),
		prop("grid-template-columns", "repeat(3, minmax(0, 1fr))"),
		prop("gap", "0.75rem 2rem"),
		prop("font-variant-numeric", "tabular-nums"),
	)
	rule(".acct-fig",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.2rem"),
		prop("min-width", "0"),
	)
	rule(".acct-fig-label",
		prop("font-size", "var(--type-12)"),
		prop("color", "var(--text-dim)"),
	)
	rule(".acct-flow-value",
		prop("font-size", "var(--type-18)"),
		prop("font-weight", "500"),
		prop("line-height", "1.25"),
	)

	// The left and right columns stack their items with one even rhythm.
	rule(".acct-panel-chart, .acct-panel-side",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.9rem"),
		prop("min-width", "0"),
	)
	// The projection sits under the history chart as a continuation, so it drops its own divider.
	rule(".acct-panel-chart .acct-fwd",
		prop("border-top", "0"),
		prop("margin-top", "0"),
		prop("padding-top", "0"),
	)

	// The footer: documents and the institution nudge on one line. An open documents drawer
	// takes the full line, and the institution action sits at the far edge.
	rule(".acct-panel-footer",
		prop("grid-column", "1 / -1"),
		prop("display", "flex"),
		prop("flex-wrap", "wrap"),
		prop("align-items", "center"),
		prop("gap", "0.75rem 1.5rem"),
		prop("min-width", "0"),
	)
	rule(".acct-panel-footer > .acct-docs:has(> .acct-docs-body)",
		prop("flex-basis", "100%"),
	)
	rule(".acct-panel-footer .acct-set-institution",
		prop("margin-left", "auto"),
	)

	// Tool buttons in the panel keep their natural width instead of stretching to the column.
	rule(".acct-row-details .btn-tool",
		prop("width", "auto"),
		prop("align-self", "flex-start"),
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
}
