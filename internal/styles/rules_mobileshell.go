// SPDX-License-Identifier: MIT

package styles

// registerMobileShell emits the phone-shell rules (390×844): the top bar wraps
// its context strip onto its own row instead of crushing title/chips into
// overlapping zero-width boxes; the icon-only rail hides the prose that can't
// survive a 56px column and is REMOVED entirely below the tab-bar breakpoint
// so the two navigation systems never coexist (UX-01); and the bottom bar is
// five fixed slots plus a floating quick-add and a More bottom sheet. Theme
// tokens only.
func registerMobileShell() {
	// ── Top bar: two-row layout on phones ────────────────────────────────────
	ruleMedia("(max-width: 640px)", ".topbar",
		flexWrap("wrap"),
		height("auto"),
		rowGap("0.15rem"),
		paddingTop("0.4rem"),
		paddingBottom("0.4rem"),
	)
	// Row 1 = title + actions; row 2 = the context strip, full-width and
	// horizontally scrollable (period control, sample chip, updated stamp).
	ruleMedia("(max-width: 640px)", ".topbar .tb-title",
		order("0"),
		flex("1 1 auto"),
	)
	ruleMedia("(max-width: 640px)", ".topbar .tb-actions",
		order("1"),
		marginLeft("auto"),
	)
	ruleMedia("(max-width: 640px)", ".topbar .tb-context",
		order("2"),
		flexBasis("100%"),
		minWidth("100%"),
		overflowX("auto"),
		overflowY("hidden"),
		flexShrink("0"),
		scrollbarWidth("none"),
	)

	// ── Icon-only rail: no prose in a 56px column ────────────────────────────
	ruleMedia("(max-width: 768px)", "aside.rail .rail-foot-info, aside.rail .ws-switch-head, aside.rail .ws-switch-value",
		display("none"),
	)
	// The workspace-switch trigger with its text hidden is just an unlabeled
	// dark block under the logo — hide the whole control in icon-rail mode.
	ruleMedia("(max-width: 768px)", "aside.rail .ws-switch",
		display("none"),
	)
	// Below the tab-bar breakpoint the rail goes away completely: the bottom
	// bar is the ONLY navigation, so no dual-nav and no dead 56px column.
	ruleMedia("(max-width: 640px)", "aside.rail",
		display("none !important"),
	)

	// ── Floating quick-add above the bar ─────────────────────────────────────
	rule(".mobile-tab-fab",
		display("none"),
	)
	ruleMedia("(max-width: 640px)", ".mobile-tab-fab",
		display("flex"),
		alignItems("center"),
		justifyContent("center"),
		position("fixed"),
		right("16px"),
		bottom("calc(56px + env(safe-area-inset-bottom, 0px) + 16px)"),
		width("52px"),
		height("52px"),
		borderRadius("50%"),
		border("0"),
		background("var(--accent, #2e8b57)"),
		color("#fff"),
		boxShadow("0 4px 14px rgba(0, 0, 0, 0.35)"),
		cursor("pointer"),
		zIndex("21"),
	)

	// ── More bottom sheet + backdrop ─────────────────────────────────────────
	rule(".mobile-more-backdrop, .mobile-more-sheet",
		display("none"),
	)
	ruleMedia("(max-width: 640px)", ".mobile-more-backdrop",
		display("block"),
		position("fixed"),
		inset("0"),
		background("rgba(0, 0, 0, 0.45)"),
		zIndex("22"),
	)
	ruleMedia("(max-width: 640px)", ".mobile-more-sheet",
		display("flex"),
		flexDirection("column"),
		position("fixed"),
		left("0"),
		right("0"),
		bottom("calc(56px + env(safe-area-inset-bottom, 0px))"),
		background("var(--bg-elev, #1a1a1d)"),
		borderTop("1px solid var(--border, #2a2a2c)"),
		borderRadius("16px 16px 0 0"),
		padding("0.5rem 0 0.6rem"),
		maxHeight("60vh"),
		overflowY("auto"),
		zIndex("23"),
		boxShadow("0 -8px 24px rgba(0, 0, 0, 0.35)"),
	)
	ruleMedia("(max-width: 640px)", ".mobile-sheet-item",
		display("flex"),
		alignItems("center"),
		gap("0.75rem"),
		padding("0.75rem 1.1rem"),
		minHeight("48px"),
		color("var(--text, #f4f4f5)"),
		fontSize("0.9rem"),
		textDecoration("none"),
		cursor("pointer"),
	)
	ruleMedia("(max-width: 640px)", ".mobile-sheet-item.active",
		color("var(--accent, #2e8b57)"),
		fontWeight("600"),
	)
	// At 320px each of the five slots is only 64px wide — "Transactions" needs
	// a slightly smaller label to fit without clipping.
	ruleMedia("(max-width: 360px)", ".mobile-tabbar .mobile-tab-label",
		fontSize("0.56rem"),
		letterSpacing("0"),
	)
}
