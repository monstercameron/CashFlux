// SPDX-License-Identifier: MIT

package styles

// registerImportWizard emits the styles for the transactions Import wizard's Stage 1
// "Add your data" picker: two branches (Smart = deterministic helpers, Smart+ =
// generative AI) each showing a grid of document-type tiles. Clicking a tile swaps the
// grid for that source's form, which animates in via docFormIn.
func registerImportWizard() {
	// Picker shell + branch sections.
	rule(".doc-picker",
		display("flex"),
		flexDirection("column"),
		gap("1.5rem"),
	)
	rule(".doc-branch",
		display("flex"),
		flexDirection("column"),
		gap("0.75rem"),
	)
	rule(".doc-branch-head",
		display("flex"),
		alignItems("baseline"),
		gap("0.6rem"),
		flexWrap("wrap"),
	)
	rule(".doc-branch-title",
		fontSize("0.95rem"),
		fontWeight("600"),
		letterSpacing("0.02em"),
		color("var(--text)"),
	)
	// Smart+ (generative AI) branch is tinted with the brand/violet accent so the two
	// tiers read as distinct at a glance.
	rule(".doc-branch-title.plus",
		color("var(--brand)"),
	)
	rule(".doc-branch-sub",
		fontSize("0.8rem"),
		color("var(--text-dim)"),
	)
	rule(".doc-tier-pill",
		fontSize("0.68rem"),
		fontWeight("600"),
		letterSpacing("0.04em"),
		textTransform("uppercase"),
		padding("0.15rem 0.5rem"),
		borderRadius("999px"),
		border("1px solid var(--border)"),
		color("var(--text-dim)"),
	)
	rule(".doc-tier-pill.plus",
		color("var(--brand)"),
		border("1px solid rgba(124,131,255,0.4)"),
		background("rgba(124,131,255,0.08)"),
	)

	// The tile grid + the tiles themselves.
	rule(".doc-type-grid",
		display("grid"),
		gridTemplateColumns("repeat(auto-fit,minmax(220px,1fr))"),
		gap("0.75rem"),
	)
	rule(".doc-type-tile",
		display("flex"),
		alignItems("flex-start"),
		gap("0.75rem"),
		textAlign("left"),
		width("100%"),
		padding("1rem"),
		border("1px solid var(--border)"),
		borderRadius("12px"),
		background("var(--bg-card)"),
		color("var(--text)"),
		cursor("pointer"),
		position("relative"),
		transition("transform 0.16s ease, border-color 0.16s ease, box-shadow 0.16s ease, background 0.16s ease"),
	)
	rule(".doc-type-tile:hover",
		transform("translateY(-2px)"),
		borderColor("var(--accent)"),
		boxShadow("0 6px 20px rgba(0,0,0,0.25)"),
	)
	rule(".doc-type-tile:focus-visible",
		outline("2px solid var(--accent)"),
		outlineOffset("2px"),
	)
	rule(".doc-type-tile.smartplus:hover",
		borderColor("var(--brand)"),
		boxShadow("0 6px 20px rgba(124,131,255,0.22)"),
	)
	rule(".doc-type-tile .dt-icon",
		flex("0 0 auto"),
		width("2.5rem"),
		height("2.5rem"),
		borderRadius("10px"),
		display("flex"),
		alignItems("center"),
		justifyContent("center"),
		background("var(--accent-dim)"),
		color("var(--accent)"),
	)
	rule(".doc-type-tile.smartplus .dt-icon",
		background("rgba(124,131,255,0.14)"),
		color("var(--brand)"),
	)
	rule(".doc-type-tile .dt-body",
		display("flex"),
		flexDirection("column"),
		gap("0.2rem"),
		minWidth("0"),
	)
	rule(".doc-type-tile .dt-title",
		fontWeight("600"),
		fontSize("0.95rem"),
		color("var(--text)"),
	)
	rule(".doc-type-tile .dt-desc",
		fontSize("0.8rem"),
		color("var(--text-dim)"),
		lineHeight("1.35"),
	)
	rule(".doc-type-tile .dt-badge",
		position("absolute"),
		top("0.6rem"),
		right("0.6rem"),
		color("var(--brand)"),
	)

	// The selected source's form animates in over the grid it replaced.
	rule(".doc-form",
		animation("docFormIn 0.24s cubic-bezier(0.2,0.7,0.2,1)"),
	)
	rule(".doc-form-head",
		display("flex"),
		alignItems("center"),
		marginBottom("0.85rem"),
	)
	keyframes("docFormIn",
		at("from", opacity("0"), transform("translateY(10px) scale(0.985)")),
		at("to", opacity("1"), transform("none")),
	)
	// Respect reduced-motion: skip the entrance animation.
	ruleMedia("(prefers-reduced-motion:reduce)", ".doc-form",
		animation("none"),
	)
}
