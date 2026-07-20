// SPDX-License-Identifier: MIT

package styles

// registerUxbatch5 holds the CSS for UX review batch #5 (2026-07-19): the
// notifications severity label's light-theme contrast, the annual-report section
// index's overflow scroll cue, and the spending-digest select's width so the
// Alerts quiet-hours block reads as one system. Registered AFTER
// registerGenerated() (and after the reports/notif rule sets) so these
// equal-or-higher-specificity refinements win the cascade. Theme tokens only.
func registerUxbatch5() {
	// ── #2 Notifications severity label — light-theme contrast ────────────────
	// `.notif-sev-tag.sev-warning` painted a pale amber (#fcd34d) with NO light
	// override, so on the light notifications card (cream) the "WARNING" caption
	// under each alert rendered at ~1.8:1 — invisible. `.sev-critical` had the same
	// gap (#fca5a5 pale red). Dark mode keeps the bright tone (readable on the dark
	// card); light mode swaps to the same deep amber/red the other severity chips
	// already use ([data-theme="light"] .sev-warning = #92400e, .sev-critical =
	// #991b1b), clearing WCAG-AA on the tinted card. The [data-theme] prefix raises
	// specificity above the base `.notif-sev-tag.sev-*` so this wins in light only.
	rule("[data-theme=\"light\"] .notif-sev-tag.sev-warning",
		color("#92400e"),
	)
	rule("[data-theme=\"light\"] .notif-sev-tag.sev-critical",
		color("#991b1b"),
	)

	// ── #4 Annual-report section index — overflow scroll cue ──────────────────
	// Under ~1280px content the jump index (.rpta-index) becomes a single
	// non-wrapping row with overflow-x:auto (rules_reportsannual.go). It clipped the
	// last chip mid-word ("09 Pr…") at the right edge with no signal that more
	// sections lay off-screen. A right-edge fade mask makes the cutoff read as
	// "scroll for more" instead of a broken chip. Applied only inside the same
	// overflow band (wider panes wrap and need no cue), and the mask is a hair's
	// width so the sticky card's body/shadow stay opaque except at the very edge.
	ruleContentMax(1280-railCollapsedPx, ".rpta-index",
		prop("-webkit-mask-image", "linear-gradient(to right, #000 calc(100% - 2rem), transparent)"),
		prop("mask-image", "linear-gradient(to right, #000 calc(100% - 2rem), transparent)"),
		prop("padding-right", "2rem"),
	)

	// ── #5 Spending-digest select — width consistency with quiet hours ────────
	// Batch #4 sized the quiet-hours From/Until time inputs to content, but the
	// sibling "Spending digest" cadence select (its own component, a different
	// testid) kept the base `.set-input` width:100% and spanned the whole row, so
	// the Alerts block read as two systems. Give it the same compact, left-aligned
	// treatment so the three controls line up as one.
	rule("[data-testid=\"settings-digest-cadence\"] .toggle-row",
		prop("justify-content", "flex-start"),
		prop("gap", "0.75rem"),
	)
	rule("[data-testid=\"settings-digest-cadence\"] .toggle-row > span",
		prop("flex", "none"),
		prop("white-space", "nowrap"),
	)
	rule("[data-testid=\"settings-digest-cadence\"] .set-input",
		prop("width", "auto"),
		prop("flex", "none"),
		prop("min-width", "9rem"),
	)
}
