// SPDX-License-Identifier: MIT

package styles

// registerImportWizard emits the styles for the transactions Import wizard's Stage 1
// "Add your data" picker: two branches (Smart = deterministic, Smart+ = generative AI)
// each showing a grid of document-type tiles. Clicking a tile swaps the grid for that
// source's form, which animates in via docFormIn. Tier hues come from the project-wide
// tier tokens (see rules_tier.go); this file only lays them out.
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
	)
	// Each branch title carries its tier hue so Smart and Smart+ read as distinct
	// product tiers at a glance.
	rule(".doc-branch-title.smart",
		color("var(--tier-smart)"),
	)
	rule(".doc-branch-title.plus",
		color("var(--tier-smartplus)"),
	)
	rule(".doc-branch-sub",
		fontSize("var(--type-13)"),
		color("var(--text-dim)"),
	)

	// The tile grid + the tiles themselves. Icons use the shared .tier-icon primitive.
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
		borderRadius("var(--radius-xl)"),
		background("var(--bg-card)"),
		color("var(--text)"),
		cursor("pointer"),
		position("relative"),
		transition("transform 0.16s ease, border-color 0.16s ease, box-shadow 0.16s ease, background 0.16s ease"),
	)
	rule(".doc-type-tile:hover",
		transform("translateY(-2px)"),
		boxShadow("0 6px 20px rgba(0,0,0,0.25)"),
	)
	rule(".doc-type-tile.smart:hover",
		borderColor("var(--tier-smart)"),
	)
	rule(".doc-type-tile.smartplus:hover",
		borderColor("var(--tier-smartplus)"),
		boxShadow("0 6px 20px rgba(124,131,255,0.22)"),
	)
	rule(".doc-type-tile.smart:focus-visible",
		outline("2px solid var(--tier-smart)"),
		outlineOffset("2px"),
	)
	rule(".doc-type-tile.smartplus:focus-visible",
		outline("2px solid var(--tier-smartplus)"),
		outlineOffset("2px"),
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
		fontSize("var(--type-13)"),
		color("var(--text-dim)"),
		lineHeight("1.35"),
	)

	// The selected source's form animates in over the grid it replaced.
	rule(".doc-form",
		animation("docFormIn var(--motion-layout) var(--ease-enter)"),
	)
	rule(".doc-form-head",
		display("flex"),
		alignItems("center"),
		marginBottom("0.85rem"),
	)
	// Per-source form header: the tier icon + title + tier chip + one-line description,
	// tying the form back to the tile that opened it.
	rule(".doc-form-title",
		display("flex"),
		alignItems("flex-start"),
		gap("0.75rem"),
		marginBottom("1.1rem"),
	)
	rule(".doc-form-titletext",
		display("flex"),
		flexDirection("column"),
		gap("0.25rem"),
		minWidth("0"),
	)
	rule(".doc-form-h",
		display("flex"),
		alignItems("center"),
		gap("0.5rem"),
		flexWrap("wrap"),
		margin("0"),
		fontSize("1.05rem"),
		fontWeight("600"),
		color("var(--text)"),
	)
	rule(".doc-form-desc",
		margin("0"),
		fontSize("var(--type-14)"),
		color("var(--text-dim)"),
		lineHeight("1.45"),
		maxWidth("62ch"),
	)
	// A tidy vertical rhythm for the inputs inside a source form.
	rule(".doc-form-body",
		display("flex"),
		flexDirection("column"),
		gap("0.75rem"),
	)
	// Direct button children shouldn't stretch to the full form width (the flex column
	// stretches children by default) — size them to their label instead.
	rule(".doc-form-body > button",
		prop("align-self", "flex-start"),
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
