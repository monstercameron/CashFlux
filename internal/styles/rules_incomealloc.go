// SPDX-License-Identifier: MIT

package styles

// registerIncomeAllocation styles the /budgets income-allocation read that sits
// directly beneath the hero band: how much of your income you have budgeted.
//
// It deliberately borrows the existing .zbb-alloc bar, marker and legend rather
// than introducing a second visual language for the same idea — the only new
// chrome is the caption line above it, which is what turns two numbers into a
// stated relationship. Registered after the generated sheet so equal-specificity
// refinements win.
func registerIncomeAllocation() {
	// The block is separated from the band by a hairline rather than a gap: they
	// are two readings of one plan, not two unrelated cards.
	// The block is inset to the band's own gutter so the bar starts and ends
	// where the figures above it do — a full-bleed bar under a padded band reads
	// as a different container's content.
	rule(".budget-alloc",
		marginTop("-0.35rem"),
		marginBottom("1rem"),
		padding("0.85rem 1.5rem 0"),
		borderTop("1px solid var(--border)"),
	)
	// Money past the income tick is money that does not exist. It wears the same
	// stripe the spend band uses for an overage, so one visual language means
	// "past the line" everywhere on this page.
	rule(".zbb-alloc-seg.is-overflow",
		prop("background", "repeating-linear-gradient(45deg,"+
			" color-mix(in srgb, var(--danger) 55%, transparent) 0,"+
			" color-mix(in srgb, var(--danger) 55%, transparent) 6px,"+
			" color-mix(in srgb, var(--danger) 22%, transparent) 6px,"+
			" color-mix(in srgb, var(--danger) 22%, transparent) 12px)"),
	)
	// Caption row: the eyebrow on the left, the change-income control on the right.
	rule(".budget-alloc-cap",
		display("flex"),
		alignItems("baseline"),
		flexWrap("wrap"),
		gap("0.2rem 0.5rem"),
	)
	// The reading line. Percentage, its denominator and the relationship sit
	// ADJACENT, in that order, so the number is never separated from what it is a
	// percentage OF.
	rule(".budget-alloc-line",
		display("flex"),
		alignItems("baseline"),
		flexWrap("wrap"),
		gap("0.15rem 0.45rem"),
		marginTop("0.15rem"),
	)
	rule(".budget-alloc-denom",
		fontSize("var(--type-13)"),
		color("var(--text-dim)"),
	)
	// The separator is attached to the relation rather than being its own element:
	// a standalone middot orphans onto its own line when the caption wraps at
	// narrow widths, which reads as a rendering fault.
	rule(".budget-alloc-relation::before",
		prop("content", `"· "`),
		color("var(--text-faint)"),
	)
	rule(".budget-alloc-cap-label",
		fontSize("0.66rem"),
		fontWeight("700"),
		letterSpacing("0.05em"),
		textTransform("uppercase"),
		color("var(--text-faint)"),
	)
	// The percentage is the figure the eye should land on, so it carries the
	// weight the caption's other parts deliberately do not.
	rule(".budget-alloc-pct",
		fontSize("1.05rem"),
		fontWeight("800"),
		lineHeight("1"),
		color("var(--text)"),
	)
	rule(".budget-alloc-relation",
		fontSize("var(--type-13)"),
		color("var(--text-dim)"),
	)
	// Over income is the one state worth a tone. It is a statement of fact, not
	// an alarm — the bar's marker already carries the visual signal.
	rule(".budget-alloc-relation.is-over",
		color("var(--money-negative)"),
		fontWeight("600"),
	)
	// The allocation bar is deliberately SLIMMER than the spend band above it.
	// They are both horizontal bars but they have different denominators — spent-
	// of-budget versus budgeted-of-income — and identical styling invited the
	// reader to apply one reading to the other.
	rule(".budget-alloc-bar",
		height("10px"),
	)
	// The caveat that travels with the hero figure when the plan runs past income.
	rule(".budget-loader-caveat",
		marginTop("0.15rem"),
		fontSize("var(--type-12)"),
		fontWeight("600"),
		color("var(--money-negative)"),
		textAlign("right"),
	)
	// A quiet text control, not a button: changing the basis is a settings act,
	// and it must not compete with the figures beside it.
	rule(".budget-alloc-change",
		fontSize("var(--type-12)"),
		fontWeight("600"),
		color("var(--accent)"),
		background("none"),
		border("0"),
		padding("0"),
		cursor("pointer"),
		prop("text-decoration", "underline"),
		prop("text-underline-offset", "2px"),
	)
	rule(".budget-alloc-change:hover", color("var(--text)"))
	// At narrow widths the caption stacks; the income figure and its control keep
	// their own line so the percentage and the relationship stay adjacent.
	ruleMedia("(max-width: 640px)", ".budget-alloc-cap > span:last-child",
		prop("margin-left", "0"),
		prop("width", "100%"),
	)
}

// registerBudgetCatOptIn styles the add-a-budget category row: the picker for
// what the budget watches, and the opt-in that replaces it with a new category.
func registerBudgetCatOptIn() {
	rule(".budget-cat-row",
		display("grid"),
		gap("0.5rem"),
	)
	// The opt-in reads as a quiet alternative under the picker, not as a second
	// field of equal weight.
	rule(".budget-cat-optin",
		fontSize("var(--type-13)"),
		color("var(--text-dim)"),
		cursor("pointer"),
	)
	rule(".budget-cat-optin input[disabled]",
		cursor("not-allowed"),
		opacity("0.6"),
	)
	rule(".budget-cat-optin:hover", color("var(--text)"))
}
