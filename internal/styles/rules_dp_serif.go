// SPDX-License-Identifier: MIT

package styles

// registerDpSerif constrains the editorial Fraunces display serif so it stays a
// deliberate accent instead of the default face for every number on screen.
//
// From the frontend-design review: the serif is an asset, but it had crept onto
// too many *operational* readouts (stat figures, tile totals, insight metrics,
// per-card values), which dilutes it. The serif should speak only for page
// titles, ONE hero value/statement per page, and major report section titles.
//
// This file touches ONLY `font-family`: it overrides the specific operational
// value/amount/figure classes that currently render in the display serif back to
// the app's sans body stack, and it deliberately LEAVES the heroes, titles, and
// report section titles serif. It does NOT touch `--font-display` globally, so
// the kept-serif classes are unaffected.
//
// Registered last (from Register()) so these equal-or-higher-specificity
// font-family overrides win the cascade over the generated/surface rules.
func registerDpSerif() {
	// The app body/sans stack, copied verbatim from the generated `body` rule so
	// the demoted figures match every other operational number in the app.
	const sans = "var(--font-ui), Inter, ui-sans-serif, system-ui, -apple-system, \"Segoe UI\", Roboto, sans-serif"
	// The display serif, re-asserted only where a hero must be clawed back from a
	// base override (see .budget-loader-value.is-hero below).
	const serif = "var(--font-display), Fraunces, Georgia, serif"

	// ── Operational value/amount/figure classes → sans ──────────────────────────
	// Each selector matches the specificity of the rule that set the serif, so the
	// later registration order decides the tie and this override wins.

	// Budgets: per-tile summary "loader" figures. The hero variant is re-asserted
	// serif below; only the everyday (non-hero) figures demote to sans.
	rule(".budget-loader-value",
		prop("font-family", sans),
	)
	// Budgets: age-of-money insight metric (explicitly "not a hero").
	rule(".budget-agemoney-num",
		prop("font-family", sans),
	)
	// Saved views: per-tile total figure (repeated across tiles).
	rule(".saved-view-tile-total",
		prop("font-family", sans),
	)
	// Debt page: the ranking numeral and the small stat-grid values.
	rule(".debt-rank",
		prop("font-family", sans),
	)
	rule(".bento-debt .stat-value",
		prop("font-family", sans),
	)
	// Notifications: the running findings count.
	rule(".notif-summary-count",
		prop("font-family", sans),
	)
	// Formula builder: the computed result value.
	rule(".fb-result-val",
		prop("font-family", sans),
	)
	// Goals: per-card percent readout, the scannable figures-grid values, and the
	// earmarked account amount.
	rule(".bento-goals .goal-card-loader .budget-pct",
		prop("font-family", sans),
	)
	rule(".goal-fig-v",
		prop("font-family", sans),
	)
	rule(".ea-acct-earmarked",
		prop("font-family", sans),
	)
	// Goal trajectory: the ETA "when" readout.
	rule(".gtj-eta .gtj-eta-when",
		prop("font-family", sans),
	)

	// ── Kept serif — re-assert the one hero clawed back by a base override ───────
	// `.budget-loader-value.is-hero` inherits its face from the base
	// `.budget-loader-value` rule (it only overrides size/weight), so the sans
	// override above would have demoted the hero too. Re-assert the serif on the
	// higher-specificity hero selector so the ONE hero figure per surface keeps the
	// editorial voice, exactly as the design brief requires.
	rule(".budget-loader-value.is-hero",
		prop("font-family", serif),
	)

	// Deliberately LEFT serif (not touched here): page <h1> titles, section/panel/
	// group/component titles, eyebrows and names; the per-page hero values
	// (.stat-value.is-hero, .inv-hero-value, .debt-hero-value, .rpt-hero-value,
	// .rec-hero-value, .bflex-num), the hero statement (.hero-quote-text), the
	// empty-state title (.bflex-empty-title), and the report masthead score.
}
