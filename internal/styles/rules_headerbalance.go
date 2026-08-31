// SPDX-License-Identifier: MIT

package styles

// registerHeaderBalance emits the UX-04 header-hierarchy rules: the page title
// gets first claim on top-bar space (it no longer shrinks before the context
// group has fully yielded — "Dashboa…" beside full-width utilities read as a
// bug), and the context strip compresses inside its own box rather than
// spilling under the action icons.
//
// Two UX-04 rules have since been removed, both because they hid a control
// behind a premise that stopped holding:
//
//   - The two-row layout (641–1535px: title + actions on row 1, context strip
//     full-width on row 2). Above 640px the bar is one row again — scope and
//     period sit inline beside the title, which is the shape the base .topbar
//     rules already describe (nowrap; .tb-context left-anchored with
//     flex-shrink:4 so it yields before the title does). Below 641px the phone
//     shell still stacks them; see registerMobileShell.
//   - The sub-1280px music fold, premised on a labeled equivalent in the ⋯ More
//     menu. That stopped holding when the music toggle moved BACK inline to
//     tb-actions (see MoreMenu), leaving narrow windows with no music control
//     anywhere while the first-run notice still pointed at "the ♪ button in the
//     top bar". Un/mute is a one-click reflex; the button keeps its slot beside
//     the notification bell at every width.
func registerHeaderBalance() {
	// Above the phone breakpoint the title refuses to shrink until it hits a
	// generous cap; everything else in the bar can compress or fold first.
	ruleMedia("(min-width: 641px)", ".topbar .tb-title",
		flexShrink("0"),
		maxWidth("40vw"),
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
