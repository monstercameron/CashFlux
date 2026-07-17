// SPDX-License-Identifier: MIT

package styles

// registerMobileShell emits the phone-shell fixes from the commercial-parity
// scan (390×844): the top bar wraps its context strip onto its own row instead
// of crushing title/chips into overlapping zero-width boxes; the icon-only
// rail hides the footer prose that can't survive a 56px column; and the bottom
// tab bar's destination strip scrolls so ALL nine primary menus stay reachable
// with the +Add slot pinned. Theme tokens only.
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

	// ── Bottom tab bar: scrollable destination strip + pinned Add ────────────
	rule(".mobile-tab-scroll",
		display("flex"),
		flex("1 1 auto"),
		minWidth("0"),
		overflowX("auto"),
		overflowY("hidden"),
		alignItems("stretch"),
		scrollbarWidth("none"),
	)
	ruleMedia("(max-width: 640px)", ".mobile-tab-scroll .mobile-tab-item",
		flex("0 0 auto"),
		minWidth("64px"),
	)
	ruleMedia("(max-width: 640px)", ".mobile-tabbar .mobile-tab-add",
		flex("0 0 auto"),
		borderLeft("1px solid var(--border)"),
	)
}
