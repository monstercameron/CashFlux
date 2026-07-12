// SPDX-License-Identifier: MIT

package styles

// registerTierSystem emits the project-wide Smart / Smart+ tier design system: the
// brand tokens and the reusable .tier-chip / .tier-icon primitives. "Smart" is the
// deterministic tier (green); "Smart+" is the generative-AI tier (violet). The hues are
// fixed — not tied to the user-customizable accent — so the tier identity stays stable
// across themes and accents wherever it's used (import wizard, Smart hub, AI actions, …).
func registerTierSystem() {
	rule(":root",
		customProp("--tier-smart", "#2e8b57"),
		customProp("--tier-smart-soft", "rgba(46,139,87,0.14)"),
		customProp("--tier-smart-border", "rgba(46,139,87,0.40)"),
		customProp("--tier-smartplus", "#7c83ff"),
		customProp("--tier-smartplus-soft", "rgba(124,131,255,0.14)"),
		customProp("--tier-smartplus-border", "rgba(124,131,255,0.40)"),
	)

	// .tier-chip — a small uppercase pill marking a surface as Smart or Smart+. The
	// glyph inside inherits currentColor, so it takes the tier hue automatically.
	rule(".tier-chip",
		display("inline-flex"),
		alignItems("center"),
		gap("0.3rem"),
		fontSize("0.68rem"),
		fontWeight("600"),
		letterSpacing("0.04em"),
		textTransform("uppercase"),
		padding("0.15rem 0.5rem"),
		borderRadius("999px"),
		border("1px solid var(--border)"),
		color("var(--text-dim)"),
		whiteSpace("nowrap"),
	)
	rule(".tier-chip.smart",
		color("var(--tier-smart)"),
		borderColor("var(--tier-smart-border)"),
		background("var(--tier-smart-soft)"),
	)
	rule(".tier-chip.plus",
		color("var(--tier-smartplus)"),
		borderColor("var(--tier-smartplus-border)"),
		background("var(--tier-smartplus-soft)"),
	)

	// .tier-icon — a tinted rounded square holding a source/feature glyph, tier-colored.
	rule(".tier-icon",
		flex("0 0 auto"),
		width("2.5rem"),
		height("2.5rem"),
		borderRadius("10px"),
		display("flex"),
		alignItems("center"),
		justifyContent("center"),
	)
	rule(".tier-icon.smart",
		background("var(--tier-smart-soft)"),
		color("var(--tier-smart)"),
	)
	rule(".tier-icon.plus",
		background("var(--tier-smartplus-soft)"),
		color("var(--tier-smartplus)"),
	)

	// .btn-plus — the Smart+ action button: a generative-AI action reads as violet
	// (with the ✦ glyph), distinct from the green deterministic primary.
	rule(".btn.btn-plus",
		color("#fff"),
		background("var(--tier-smartplus)"),
		borderColor("var(--tier-smartplus)"),
	)
	rule(".btn.btn-plus:hover",
		background("#6b72f0"),
		borderColor("#6b72f0"),
	)
	rule(".btn.btn-plus:disabled",
		opacity("0.45"),
		cursor("not-allowed"),
	)
}
