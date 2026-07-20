// SPDX-License-Identifier: MIT

package styles

// registerUxbatch4 holds the CSS for UX review batch #4 (subscriptions/bills
// kebab overflow, notifications destructive parity, settings alert thresholds).
// It must be registered AFTER registerGenerated() so its rules win the cascade
// over the base .toggle-row declarations they refine.
func registerUxbatch4() {
	// The ⋯ menu's relocated ambient cluster (activity stamp + smart peek) sits
	// under a small caption and a hairline, separated from the labeled rows
	// below (UI/UX task #25).
	rule(".tb-more-quick-wrap",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.25rem"),
		prop("padding", "0.15rem 0.6rem 0.45rem"),
		prop("border-bottom", "1px solid var(--border-subtle)"),
		prop("margin-bottom", "0.25rem"),
	)
	rule(".tb-more-quick-label",
		prop("letter-spacing", "0.05em"),
		prop("text-transform", "uppercase"),
		prop("font-size", "var(--type-11)"),
	)

	// The gwc DEV RUNNER's floating status bubble (never present in production
	// builds) parks itself bottom-left — exactly over the rail footer's privacy
	// assurance, which the UI/UX review misread as truncated copy (task #11).
	// Development happens on `gwc dev` all day, so shepherd the bubble to the
	// right edge, above the back-to-top FAB's corner.
	// !important: the runner positions the bubble via inline styles.
	rule("#gwc-status-icon",
		prop("left", "auto !important"),
		prop("right", "18px !important"),
		prop("bottom", "84px !important"),
	)

	// The rail's nav list scrolls above the pinned Cloud/household/footer block,
	// but nothing SAID so — an expanded section's last items (e.g. the active
	// "Planning") sat half-cut at the invisible scroll boundary and read as
	// hidden behind the Cloud card (UI/UX task #4). Two affordances: a thin,
	// always-available scrollbar on the list, and a hairline seam before the
	// pinned block so "the list ends here, below is pinned" is explicit.
	rule(".rail-nav",
		prop("scrollbar-width", "thin"),
		prop("scrollbar-color", "color-mix(in srgb, var(--text) 22%, transparent) transparent"),
		prop("scrollbar-gutter", "stable"),
	)
	rule(".rail-nav + *",
		prop("border-top", "1px solid var(--border-subtle)"),
	)

	// The Budget-settings popover's method select truncated its own value
	// ("Simple (per-category limits") at the 240px menu width (UI/UX task #6).
	// Registered after registerBudgetRefine, so this wins the min-width.
	rule(".bud-set-menu",
		prop("min-width", "290px"),
	)


	// Settings → Alerts (review #29): the per-rule threshold input used a bare
	// `.toggle-row`, which is `justify-content: space-between` — so the number input
	// floated to the far right edge, a screen-width from its "$"/"days" label. Give
	// the threshold row its own compact, left-aligned layout indented under the alert
	// toggle it belongs to, matching the "Freshness reminders" (.rate-row) rows. The
	// compound `.toggle-row.alert-thresh-row` selector outranks the base `.toggle-row`
	// regardless of registration order; the shared `.toggle-row .rate-in` 90px width
	// rule still applies because the row keeps its `toggle-row` class.
	rule(".toggle-row.alert-thresh-row",
		prop("justify-content", "flex-start"),
		prop("gap", "0.6rem"),
		prop("padding", "0.1rem 0 0.55rem 1.5rem"),
		prop("border-bottom", "none"),
		prop("font-size", "var(--type-14)"),
	)
}
