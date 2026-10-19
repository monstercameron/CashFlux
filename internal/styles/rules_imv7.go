// SPDX-License-Identifier: MIT

package styles

// registerIMV7 tightens the Import modal's visual system after the shared
// component rules, so its picker, Smart+ actions, and draft review read as one
// calm, accessible flow.
func registerIMV7() {
	// Normalize every import control and review label first. Later rules below
	// assign the figure, title, and meta steps where hierarchy needs them.
	rule(".doc-picker, .doc-picker *, .doc-form, .doc-form *, .card-step-active, .card-step-active *",
		fontFamily("Inter, sans-serif"),
		fontSize("var(--fs-body)"),
		fontWeight("400"),
		fontStyle("normal"),
		letterSpacing("normal"),
		textTransform("none"),
	)
	rule(".doc-form button, .card-step-active button",
		fontWeight("500"),
	)

	// Both branches use the same tracks. The empty third Smart+ track is
	// intentional: tile width stays consistent across the two source groups.
	rule(".doc-type-grid",
		display("grid"),
		gridTemplateColumns("repeat(3, minmax(0, 1fr))"),
		alignItems("stretch"),
		gap("0.75rem"),
	)
	rule(".doc-type-tile",
		boxSizing("border-box"),
		minWidth("0"),
		minHeight("10rem"),
		height("100%"),
		alignItems("flex-start"),
	)
	rule(".doc-type-tile .dt-body",
		alignSelf("stretch"),
	)
	rule(".doc-type-tile .dt-title, .doc-branch-title",
		fontSize("var(--fs-figure)"),
		fontWeight("600"),
		letterSpacing("normal"),
	)
	rule(".doc-type-tile .dt-desc",
		fontSize("var(--fs-body)"),
		fontWeight("400"),
		lineHeight("1.4"),
		display("-webkit-box"),
		prop("-webkit-box-orient", "vertical"),
		prop("-webkit-line-clamp", "3"),
		overflow("hidden"),
	)
	rule(".doc-branch-sub, .doc-form-desc",
		fontSize("var(--fs-body)"),
		fontWeight("400"),
	)
	rule(".doc-form-h",
		fontSize("var(--fs-title)"),
		fontWeight("600"),
	)
	rule(".doc-picker .tier-chip, .doc-form .tier-chip",
		fontSize("var(--fs-meta)"),
		fontWeight("500"),
		letterSpacing("normal"),
		textTransform("none"),
	)

	// Keep the Smart+ identity visible while giving AI an outlined secondary
	// action treatment. The small mark is part of the control, not an extra CTA.
	rule(".doc-form .btn-plus, [data-testid='suggest-categories-ai']",
		color("var(--tier-smartplus)"),
		background("transparent"),
		border("1px solid var(--tier-smartplus)"),
		boxShadow("none"),
		fontSize("var(--fs-body)"),
		fontWeight("500"),
	)
	rule(".doc-form .btn-plus::before, [data-testid='suggest-categories-ai']::before",
		content("\"Smart+\""),
		fontSize("var(--fs-meta)"),
		fontWeight("600"),
		marginRight("0.25rem"),
	)
	rule(".doc-form .btn-plus .key-gate-mark, [data-testid='suggest-categories-ai'] .key-gate-mark",
		fontSize("var(--fs-meta)"),
		fontWeight("500"),
		color("var(--tier-smartplus)"),
	)
	rule(".doc-form .btn-plus:hover:not(:disabled), [data-testid='suggest-categories-ai']:hover:not(:disabled)",
		color("var(--tier-smartplus)"),
		background("var(--tier-smartplus-soft)"),
		borderColor("var(--tier-smartplus)"),
	)
	rule(".doc-form .notice.notice-warn",
		color("var(--text)"),
		fontSize("var(--fs-body)"),
		fontWeight("400"),
	)
	rule(".doc-form .notice.notice-warn a",
		color("var(--text)"),
		fontWeight("600"),
		textDecoration("underline"),
		textUnderlineOffset("2px"),
	)

	// The draft toolbar's two controls have a consistent 8 px separation, and
	// each transaction keeps a predictable 40 px scan line.
	rule(".card-step-active :has(> [data-testid='draft-clear-all'])",
		gap("8px"),
	)
	rule(".draft-ledger .row",
		minHeight("40px"),
		fontSize("var(--fs-body)"),
		fontWeight("400"),
	)
	rule(".draft-ledger .row .amount",
		fontSize("var(--fs-body)"),
		fontWeight("500"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".draft-ledger .row-meta, .draft-ledger .draft-cat-chip, .draft-ledger .draft-cat-add",
		fontSize("var(--fs-meta)"),
		fontWeight("400"),
	)

	// Tier text is darkened on the light surface and brightened on dark so the
	// green and indigo labels remain readable in either theme.
	rule(`[data-theme="light"] .doc-branch-title.smart, [data-theme="light"] .doc-picker .tier-chip.smart, [data-theme="light"] .doc-type-tile.smart .tier-icon`,
		color("#286b43"),
	)
	rule(`[data-theme="light"] .doc-branch-title.plus, [data-theme="light"] .doc-picker .tier-chip.plus, [data-theme="light"] .doc-type-tile.smartplus .tier-icon`,
		color("#4d55b8"),
	)
	rule(`[data-theme="light"] .doc-type-tile.smart:hover`,
		borderColor("#286b43"),
	)
	rule(`[data-theme="light"] .doc-type-tile.smartplus:hover`,
		borderColor("#4d55b8"),
	)
	rule(`[data-theme="light"] .doc-form .btn-plus, [data-theme="light"] [data-testid='suggest-categories-ai']`,
		color("#4d55b8"),
		borderColor("#4d55b8"),
	)
	rule(`[data-theme="light"] .doc-form .btn-plus .key-gate-mark, [data-theme="light"] [data-testid='suggest-categories-ai'] .key-gate-mark`,
		color("#4d55b8"),
	)
	rule(`[data-theme="light"] .doc-form .btn-plus:hover:not(:disabled), [data-theme="light"] [data-testid='suggest-categories-ai']:hover:not(:disabled)`,
		color("#4149a8"),
		background("rgba(77,85,184,0.08)"),
		borderColor("#4149a8"),
	)
	rule(`[data-theme="dark"] .doc-branch-title.smart, [data-theme="dark"] .doc-picker .tier-chip.smart`,
		color("#71d99a"),
	)
	rule(`[data-theme="dark"] .doc-type-tile.smart .tier-icon`,
		color("#71d99a"),
	)
	rule(`[data-theme="dark"] .doc-type-tile.smart:hover`,
		borderColor("#71d99a"),
	)
	rule(`[data-theme="dark"] .doc-branch-title.plus, [data-theme="dark"] .doc-picker .tier-chip.plus`,
		color("#aaaefe"),
	)
	rule(`[data-theme="dark"] .doc-type-tile.smartplus .tier-icon`,
		color("#aaaefe"),
	)
	rule(`[data-theme="dark"] .doc-type-tile.smartplus:hover`,
		borderColor("#aaaefe"),
	)
	rule(`[data-theme="dark"] .doc-form .btn-plus, [data-theme="dark"] [data-testid='suggest-categories-ai']`,
		color("#aaaefe"),
		borderColor("#aaaefe"),
	)
	rule(`[data-theme="dark"] .doc-form .btn-plus .key-gate-mark, [data-theme="dark"] [data-testid='suggest-categories-ai'] .key-gate-mark`,
		color("#aaaefe"),
	)

	ruleMedia("(max-width: 640px)", ".doc-type-grid",
		gridTemplateColumns("repeat(2, minmax(0, 1fr))"),
	)
	ruleMedia("(max-width: 420px)", ".doc-type-grid",
		gridTemplateColumns("1fr"),
	)
}
