// SPDX-License-Identifier: MIT

package styles

// registerPayeeCleanSurface emits the per-transaction payee-cleanup flip modal (SM-1):
// the read-only raw descriptor, the clean-name row (input + AI suggest), the scope
// control, and the footer. Theme tokens only.
func registerPayeeCleanSurface() {
	rule(".pclean",
		display("flex"),
		flexDirection("column"),
		gap("1rem"),
		padding("1.25rem"),
	)
	// The raw descriptor, shown as a quiet monospace-ish chip so it's clearly "the
	// literal string on the transaction," distinct from the editable clean name.
	rule(".pclean-raw-val",
		marginTop("0.25rem"),
		padding("0.5rem 0.7rem"),
		border("1px solid var(--border)"),
		borderRadius("8px"),
		background("color-mix(in srgb, var(--bg-elev) 45%, transparent)"),
		color("var(--text)"),
		fontFamily("var(--font-mono, ui-monospace, SFMono-Regular, Menlo, monospace)"),
		fontSize("0.82rem"),
		overflowWrap("anywhere"),
	)
	// The clean-name input fills the row; the AI suggest button trails it.
	rule(".pclean-name-row .field",
		flex("1 1 auto"),
		minWidth("0"),
	)
	rule(".pclean-scope",
		display("flex"),
		flexDirection("column"),
	)
	// Footer sits below a hairline, actions right-aligned.
	rule(".pclean-foot",
		marginTop("0.25rem"),
		paddingTop("0.85rem"),
		borderTop("1px solid color-mix(in srgb, var(--border) 60%, transparent)"),
	)

	// Rename history: the merchant's clean-name lineage as a compact bordered list —
	// the raw original at the top (monospace/muted, matching the raw chip), any prior
	// names in the middle, and the name in effect at the bottom (accented). Radius
	// matches the .pclean-raw-val chip so the two insets read as one family.
	rule(".pclean-history",
		display("flex"),
		flexDirection("column"),
		gap("0.4rem"),
	)
	rule(".pclean-history-list",
		display("flex"),
		flexDirection("column"),
		border("1px solid var(--border)"),
		borderRadius("8px"),
		overflow("hidden"),
	)
	rule(".pclean-history-item",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.6rem"),
		padding("0.4rem 0.6rem"),
		borderTop("1px solid color-mix(in srgb, var(--border) 60%, transparent)"),
		fontSize("0.85rem"),
	)
	rule(".pclean-history-list .pclean-history-item:first-child",
		borderTop("0"),
	)
	rule(".pclean-history-name",
		minWidth("0"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
		color("var(--text)"),
	)
	rule(".pclean-history-meta",
		flexShrink("0"),
		fontSize("0.72rem"),
		color("var(--text-faint)"),
		fontVariantNumeric("tabular-nums"),
	)
	// The original raw string reads like the raw chip: monospace + dimmed.
	rule(".pclean-history-item.is-raw .pclean-history-name",
		fontFamily("var(--font-mono, ui-monospace, SFMono-Regular, Menlo, monospace)"),
		color("var(--text-dim)"),
	)
	// The name in effect stands out in the accent tone.
	rule(".pclean-history-item.is-current .pclean-history-name",
		fontWeight("700"),
		color("var(--accent)"),
	)
	rule(".pclean-history-item.is-current .pclean-history-meta",
		fontWeight("600"),
		color("var(--accent)"),
	)
}
