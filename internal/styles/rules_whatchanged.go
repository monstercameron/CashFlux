// SPDX-License-Identifier: MIT

package styles

// registerWhatChanged styles the dashboard "What changed since your last visit"
// card (E-DB): a calm band in the catch-up/resume family — same card chrome —
// whose rows each read finding → amount → why → evidence, top to bottom, so the
// whole card scans in a few seconds.
func registerWhatChanged() {
	rule(".wc-card",
		border("1px solid var(--border)"),
		borderRadius("var(--radius-xl)"),
		background("var(--bg-card)"),
		padding("0.7rem 0.9rem"),
		margin("0.6rem 0"),
	)
	rule(".wc-head",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		marginBottom("0.45rem"),
		fontSize("0.9rem"),
	)
	rule(".wc-since",
		color("var(--muted)"),
		fontSize("0.8rem"),
		flex("1 1 auto"),
	)
	rule(".wc-rows",
		display("flex"),
		flexDirection("column"),
		gap("0.45rem"),
	)
	rule(".wc-row",
		display("flex"),
		alignItems("flex-start"),
		gap("0.6rem"),
	)
	rule(".wc-icon",
		fontSize("0.95rem"),
		lineHeight("1.5"),
		flex("0 0 auto"),
	)
	rule(".wc-row-body",
		flex("1 1 auto"),
		minWidth("0"),
	)
	rule(".wc-lead",
		fontSize("0.9rem"),
	)
	rule(".wc-amt",
		marginLeft("0.4rem"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".wc-why",
		color("var(--muted)"),
		fontSize("0.8rem"),
		marginTop("0.1rem"),
	)
	rule(".wc-ev",
		color("var(--text-faint)"),
		fontSize("0.78rem"),
		marginTop("0.1rem"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".wc-row .btn",
		flex("0 0 auto"),
	)
}
