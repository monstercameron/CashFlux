// SPDX-License-Identifier: MIT

package styles

// registerCoverAllSurface emits the styles for the "Cover overages in one pass"
// Smart+ modal (SMART-B14): the over-banner button and the per-over-budget rows with
// their coverage-source pickers. Theme tokens only; radii match the app's other modal
// insets (8px, like the payee-clean raw chip and the budget formula rows).
func registerCoverAllSurface() {
	// Over-banner entry button: pushed to the trailing edge of the alert.
	rule(".budget-over-banner .cover-all-open",
		marginLeft("auto"),
		flexShrink("0"),
	)

	// The list of over-budgets in the modal.
	rule(".cover-all-list",
		display("flex"),
		flexDirection("column"),
		gap("0.5rem"),
		marginTop("0.6rem"),
	)
	// One over-budget row: name + overage on the left, the source picker on the right;
	// wraps to two lines on a narrow panel.
	rule(".cover-all-row",
		display("flex"),
		alignItems("center"),
		justifyContent("space-between"),
		gap("0.75rem"),
		flexWrap("wrap"),
		padding("0.6rem 0.75rem"),
		border("1px solid var(--border)"),
		borderRadius("8px"),
		background("var(--bg-elev)"),
	)
	rule(".cover-all-row-main",
		display("flex"),
		flexDirection("column"),
		gap("0.1rem"),
		minWidth("0"),
	)
	rule(".cover-all-row-name",
		fontWeight("600"),
		color("var(--text)"),
		overflow("hidden"),
		textOverflow("ellipsis"),
		whiteSpace("nowrap"),
	)
	rule(".cover-all-row-over",
		fontSize("0.8rem"),
		fontVariantNumeric("tabular-nums"),
	)
	rule(".cover-all-row-src",
		display("flex"),
		flexDirection("column"),
		gap("0.2rem"),
		flex("1 1 13rem"),
		minWidth("11rem"),
	)
	rule(".cover-all-row-srclabel",
		fontWeight("600"),
		letterSpacing("0.03em"),
	)
	rule(".cover-all-src",
		width("100%"),
	)
	// The "borrows from next month" explainer under the list.
	rule(".cover-all-hint",
		marginTop("0.5rem"),
		lineHeight("1.45"),
	)
	rule(".cover-all-empty",
		padding("1.25rem 0"),
		textAlign("center"),
	)
}
