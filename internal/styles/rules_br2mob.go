// SPDX-License-Identifier: MIT

package styles

// registerBR2MOB fixes compact budget rows and the shared shell at the widths
// called out by the BR2MOB review.
func registerBR2MOB() {
	// Compact rows keep a stable desktop scan line while their figures remain
	// grouped together, so a narrow viewport can make the row two lines tall.
	rule(".budget-crow",
		gridTemplateColumns("minmax(8rem, 1fr) minmax(0, 1.4fr) minmax(10.5rem, auto) 8.5rem 4.6rem"),
		prop("grid-template-areas", `"name bar figures chip actions"`),
	)
	rule(".budget-crow-head", prop("grid-area", "name"), minWidth("0"))
	rule(".budget-crow-chip", prop("grid-area", "chip"), justifySelf("end"))
	rule(".budget-crow-actions", prop("grid-area", "actions"), justifySelf("end"))
	rule(".budget-crow-bar", prop("grid-area", "bar"))
	rule(".budget-crow-figures",
		prop("grid-area", "figures"),
		display("flex"),
		alignItems("center"),
		justifyContent("flex-end"),
		gap("0.15rem"),
		minWidth("0"),
	)
	rule(".budget-crow-figures .budget-crow-amt", justifySelf("auto"), textAlign("right"))
	rule(".budget-crow-figures .budget-crow-left", justifySelf("auto"), textAlign("left"))
	rule(".budget-crow-fig-separator", color("var(--text-dim)"), whiteSpace("pre"))
	rule(".budget-crow-head .budget-crow-name", flex("1 1 0"), minWidth("0"))
	rule(".budget-crow-bar .bar-pace-tick", zIndex("1"))

	// Keep the progress bar at 1024px, where the legacy content-width rule hid it.
	ruleMedia("(max-width: 1100px)", ".budget-crow-bar", display("block !important"))
	ruleMedia("(max-width: 1100px)", ".budget-crow",
		gridTemplateColumns("minmax(0, 1fr) auto auto"),
		prop("grid-template-areas", `"name chip actions" "figures figures figures"`),
		gap("0.3rem 0.55rem"),
		padding("0.5rem 0.6rem"),
	)
	ruleMedia("(max-width: 1100px)", ".budget-crow-head", minWidth("0"), overflow("hidden"))
	ruleMedia("(max-width: 1100px)", ".budget-crow-figures",
		position("relative"),
		display("flex"),
		justifyContent("flex-start"),
		paddingBottom("7px"),
		minHeight("22px"),
	)
	ruleMedia("(max-width: 1100px)", ".budget-crow-figures .budget-crow-amt",
		fontSize("var(--type-13)"),
		whiteSpace("normal"),
		textAlign("left"),
	)
	ruleMedia("(max-width: 1100px)", ".budget-crow-figures .budget-crow-left",
		fontSize("var(--type-13)"),
		whiteSpace("nowrap"),
	)
	ruleMedia("(max-width: 1100px)", ".budget-crow-bar",
		position("absolute"),
		left("0"),
		right("0"),
		bottom("0"),
		width("100%"),
		height("3px"),
	)
	ruleMedia("(max-width: 1100px)", ".budget-crow-head .budget-crow-name", flex("1 1 0"), minWidth("0"))
	ruleMedia("(max-width: 420px)", ".budget-crow-figures", gap("0"), fontSize("var(--type-12)"))
	ruleMedia("(max-width: 420px)", ".budget-crow-figures .budget-crow-amt, .budget-crow-figures .budget-crow-left",
		fontSize("var(--type-12)"),
	)

	// Budget action text is normal-sized green text, so use the theme-derived
	// AA-safe green ink rather than the decorative/fill color.
	rule(".budget-unbudgeted-cta, .budget-rollover-badge.is-on .budget-rollover-pill, .budget-rollover-badge.is-capped .budget-rollover-pill",
		color("var(--up-ink, var(--accent-ink, var(--up)))"),
	)

	// At laptop widths give the sample status its own full context row. Its
	// action stays visible and reachable instead of being clipped by the strip.
	ruleMedia("(min-width: 641px) and (max-width: 1100px)", ".topbar",
		flexWrap("wrap"),
		height("auto"),
	)
	ruleMedia("(min-width: 641px) and (max-width: 1100px)", ".topbar .tb-context",
		order("2"),
		flex("1 1 100%"),
		maxWidth("100%"),
		minWidth("0"),
		overflow("visible"),
	)
	ruleMedia("(min-width: 641px) and (max-width: 1100px)", ".topbar .sample-banner",
		flex("0 1 auto"),
		flexWrap("wrap"),
		height("auto"),
		maxWidth("100%"),
		whiteSpace("normal"),
	)
	ruleMedia("(min-width: 641px) and (max-width: 1100px)", ".topbar .sample-banner-actions",
		display("flex"),
		flexWrap("wrap"),
		minWidth("0"),
	)

	// Reserve the mobile tab bar and quick-add footprint at the end of scroll,
	// and lift Back to top above the floating quick-add button.
	ruleMedia("(max-width: 640px)", "main.cf-scroll",
		paddingBottom("calc(148px + env(safe-area-inset-bottom, 0px)) !important"),
	)
	ruleMedia("(max-width: 640px)", ".cf-scrolltop",
		bottom("calc(56px + env(safe-area-inset-bottom, 0px) + 76px)"),
	)
}
