// SPDX-License-Identifier: MIT

//go:build js && wasm

package styles

// registerHealthAnalysis emits the /health analysis-surface rules added in the
// redesign: the interactive stress-test blocks, the money-leaks read (recurring
// load + spending creep), and the active-chip state shared with the debt chips.
// Registered from install.go's Register().
func registerHealthAnalysis() {
	// Active state for the shared pill chips (base .chip-btn lives in rules_debt.go).
	rule(".chip-btn.is-active",
		borderColor("color-mix(in srgb, var(--accent) 70%, var(--border))"),
		background("color-mix(in srgb, var(--accent) 16%, var(--bg-elev))"),
		color("var(--text)"),
	)

	// --- Stress-test tile ---
	rule(".hlt-stress-lead",
		fontSize("1.05rem"),
		lineHeight("1.5"),
		prop("margin", "0.25rem 0 0.75rem"),
	)
	rule(".hlt-stress-lead b, .hlt-stress-out b",
		fontWeight("700"),
		color("var(--text)"),
	)
	rule(".hlt-stress-row",
		display("flex"),
		flexDirection("column"),
		gap("0.4rem"),
		padding("0.7rem 0"),
		prop("border-top", "1px solid var(--border)"),
	)
	rule(".hlt-stress-ctrl",
		display("flex"),
		alignItems("center"),
		flexWrap("wrap"),
		gap("0.6rem"),
	)
	rule(".hlt-stress-label",
		fontSize("var(--type-11)"),
		prop("text-transform", "uppercase"),
		letterSpacing("0.06em"),
		minWidth("6.5rem"),
	)
	rule(".hlt-stress-chips",
		display("flex"),
		flexWrap("wrap"),
		gap("0.4rem"),
	)
	rule(".hlt-stress-out",
		prop("margin", "0"),
		fontSize("0.95rem"),
		lineHeight("1.5"),
		color("var(--text-dim)"),
	)

	// --- Money-leaks tile ---
	rule(".hlt-leak-block",
		display("flex"),
		flexDirection("column"),
		gap("0.35rem"),
	)
	rule(".hlt-leak-head",
		display("flex"),
		alignItems("baseline"),
		flexWrap("wrap"),
		gap("0.5rem"),
	)
	rule(".hlt-leak-figure",
		fontSize("1.5rem"),
		fontWeight("700"),
		fontVariantNumeric("tabular-nums"),
		color("var(--text)"),
	)
	rule(".hlt-creep-row",
		display("flex"),
		alignItems("baseline"),
		justifyContent("space-between"),
		flexWrap("wrap"),
		gap("0.5rem"),
		padding("0.4rem 0"),
		prop("border-top", "1px solid var(--border)"),
	)
	rule(".hlt-creep-name",
		color("var(--text)"),
	)
	rule(".hlt-creep-save",
		fontVariantNumeric("tabular-nums"),
		fontWeight("600"),
		prop("margin-left", "auto"),
	)
}
