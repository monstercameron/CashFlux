// SPDX-License-Identifier: MIT

package styles

// registerCatSuggestSurface emits the per-transaction categorize affordance (SM-2):
// the inline row chip that files a charge in one click, and the flip modal opened
// from the row kebab. Theme tokens only.
func registerCatSuggestSurface() {
	// --- the inline row chip ---------------------------------------------------
	// It sits in the category cell of an UNCATEGORIZED row, so it has to read as an
	// offer rather than as a filed category: accent-tinted and outlined, where a
	// real category name in that column is plain text.
	rule(".txn-catsuggest",
		display("inline-flex"),
		alignItems("center"),
		gap("0.3rem"),
		maxWidth("100%"),
		padding("0.1rem 0.4rem"),
		border("1px dashed color-mix(in srgb, var(--accent) 55%, transparent)"),
		borderRadius("var(--radius-sm, 6px)"),
		background("color-mix(in srgb, var(--accent) 8%, transparent)"),
		color("var(--accent)"),
		fontSize("var(--type-12)"),
		fontWeight("600"),
		lineHeight("1.35"),
		cursor("pointer"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".txn-catsuggest:hover",
		background("color-mix(in srgb, var(--accent) 16%, transparent)"),
		borderStyle("solid"),
	)
	rule(".txn-catsuggest:focus-visible",
		outline("2px solid var(--accent)"),
		outlineOffset("1px"),
	)

	// --- the flip modal --------------------------------------------------------
	rule(".csug",
		display("flex"),
		flexDirection("column"),
		gap("1rem"),
		padding("1.25rem"),
	)
	// The charge being filed, as a quiet read-only inset — the same chip family as
	// the payee-cleanup modal's raw descriptor, so the two read as one system.
	rule(".csug-txn-val",
		display("flex"),
		alignItems("baseline"),
		justifyContent("space-between"),
		gap("0.75rem"),
		marginTop("0.25rem"),
		padding("0.5rem 0.7rem"),
		border("1px solid var(--border)"),
		borderRadius("var(--radius-lg)"),
		background("color-mix(in srgb, var(--bg-elev) 45%, transparent)"),
	)
	rule(".csug-txn-payee",
		minWidth("0"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
		color("var(--text)"),
		fontWeight("600"),
	)
	rule(".csug-txn-amt",
		flexShrink("0"),
		color("var(--text)"),
		fontVariantNumeric("tabular-nums"),
	)
	// The suggestion + its evidence. The accent tint marks it as a proposal; the
	// "why" line under it is what makes it judgeable rather than merely trusted.
	rule(".csug-suggest",
		display("flex"),
		flexDirection("column"),
		gap("0.3rem"),
		padding("0.6rem 0.7rem"),
		border("1px solid color-mix(in srgb, var(--accent) 35%, var(--border))"),
		borderRadius("var(--radius-lg)"),
		background("color-mix(in srgb, var(--accent) 6%, transparent)"),
	)
	rule(".csug-suggest-name",
		color("var(--accent)"),
		fontWeight("700"),
		fontSize("var(--type-15)"),
	)
	rule(".csug-why",
		margin("0"),
	)
	rule(".csug-none",
		margin("0"),
		padding("0.6rem 0.7rem"),
		border("1px dashed var(--border)"),
		borderRadius("var(--radius-lg)"),
	)
	// The picker fills the row; the Smart+ ask trails it, mirroring the payee
	// modal's name row so the two AI affordances sit in the same place.
	rule(".csug-pick-row .field",
		flex("1 1 auto"),
		minWidth("0"),
	)
	rule(".csug-ainote",
		margin("0"),
	)
	rule(".csug-batch",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		paddingTop("0.15rem"),
	)
}
