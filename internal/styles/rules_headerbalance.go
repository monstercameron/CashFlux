// SPDX-License-Identifier: MIT

package styles

// registerHeaderBalance emits the UX-04 header-hierarchy rules: the page title
// gets first claim on top-bar space (it no longer shrinks before the context
// group has fully yielded — "Dashboa…" beside full-width utilities read as a
// bug), the music/lock utility buttons fold out of the bar on sub-1280px
// widths (their labeled equivalents live in the ⋯ More menu; the inline
// buttons stay mounted so the music player effect keeps running), and below
// 1536px the context strip takes its own full-width second row — the two-row
// layout the original top-bar comment promised but never shipped.
func registerHeaderBalance() {
	// Above the phone breakpoint the title refuses to shrink until it hits a
	// generous cap; everything else in the bar can compress or fold first.
	ruleMedia("(min-width: 641px)", ".topbar .tb-title",
		flexShrink("0"),
		maxWidth("40vw"),
	)
	ruleMedia("(max-width: 1280px)", ".topbar .muzak-btn",
		display("none !important"),
	)
	// The freshness stamp is the context strip's least critical leg (its
	// destination, /activity, is a click away regardless) — it yields width
	// inside its own box instead of spilling under the actions divider when
	// the strip comes up a few pixels short.
	rule(".topbar .tb-context .tb-updated",
		flex("0 1 auto !important"),
		minWidth("0"),
		overflow("hidden"),
	)
	// When the scope/period context group still can't fit its full-width row,
	// it scrolls within its own box instead of spilling under the action icons
	// (the ≤640px shell already does this; mid widths lacked the rule).
	ruleMedia("(min-width: 641px) and (max-width: 1535px)", ".topbar .tb-context",
		overflowX("auto"),
		overflowY("hidden"),
		scrollbarWidth("none"),
	)
	// Two-row layout below 1536px: row 1 = title + actions, row 2 = the
	// context strip full-width — same shape the phone shell uses — so the
	// period control never hides inside a clipped scroller beside the title.
	ruleMedia("(min-width: 641px) and (max-width: 1535px)", ".topbar",
		flexWrap("wrap"),
		height("auto"),
		rowGap("0.15rem"),
		paddingTop("0.4rem"),
		paddingBottom("0.4rem"),
	)
	ruleMedia("(min-width: 641px) and (max-width: 1535px)", ".topbar .tb-title",
		order("0"),
	)
	ruleMedia("(min-width: 641px) and (max-width: 1535px)", ".topbar .tb-actions",
		order("1"),
		marginLeft("auto"),
	)
	ruleMedia("(min-width: 641px) and (max-width: 1535px)", ".topbar .tb-context",
		order("2"),
		flexBasis("100%"),
		minWidth("100%"),
		flexShrink("0"),
	)
	// On truly squeezed widths the "Viewing as" prose folds (the select keeps
	// its accessible name) and the member select tightens, so the full context
	// row fits without hiding the period control behind a scroll edge.
	ruleMedia("(max-width: 1180px)", ".topbar .cf-viewas-label",
		display("none !important"),
	)
	ruleMedia("(max-width: 1180px)", ".topbar .member-switcher",
		maxWidth("10rem"),
	)
}
