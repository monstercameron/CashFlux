// SPDX-License-Identifier: MIT

package styles

// registerDpType is the 2026-07-19 typographic-scale pass (frontend design
// review): it enforces five fixed information levels app-wide by SIZE and WEIGHT
// only, and — the key fix — lifts the faintest secondary metadata out of
// decorative-texture territory into a legible dim. It deliberately touches only
// font-size / font-weight / line-height / letter-spacing, plus the neutral gray
// COLOR of metadata (never semantic green/red/amber, and never font-family —
// other agents own those). Registered LAST in install.go so equal-specificity
// rules win the cascade over the generated base. Theme tokens only; light + dark
// both track, because the metadata colours resolve through var(--text-dim) /
// var(--text-faint), which each theme redefines.
//
// The five levels:
//  1. Page title      — 24-28px, one line          (.page-title)
//  2. Hero value      — 32-44px                     (owned elsewhere; not forced here)
//  3. Section title   — 18-20px, semibold           (.wh h2/.wh h3/.wh-title)
//  4. Row/card title  — 14-16px, semibold           (.card-title)
//  5. Metadata        — 12-13px, legible dim        (.set-label, .text-faint, .t-caption)
func registerDpType() {
	// Level 1 — Page title. 1.6rem ≈ 25.6px sits in the 24-28px band; pin the
	// weight and line-height so every page header reads at exactly one level.
	rule(".page-title",
		fontSize("1.6rem"),
		fontWeight("600"),
		lineHeight("1.2"),
	)

	// Level 3 — Section title. The bento widget header is the app's shared section
	// title; the base rule left it at 1rem (16px), a level below where a section
	// heading belongs. Raise to 1.125rem (18px) — the floor of the 18-20px band —
	// semibold, without touching font-family (serif agent owns it).
	rule(".wh h2, .wh h3, .wh .wh-title",
		fontSize("1.125rem"),
		fontWeight("600"),
		lineHeight("1.3"),
	)

	// Level 4 — Row/card title. Base was 1.05rem (16.8px); nudge to 1rem (16px) so
	// card headers land at the top of the 14-16px band and read as clearly
	// subordinate to a section title.
	rule(".card-title",
		fontSize("1rem"),
		fontWeight("600"),
		lineHeight("1.35"),
	)

	// Level 5 — Metadata legibility (the key fix). These small labels were dimmed
	// so far they read as texture rather than text:
	//   .set-label   — settings micro-labels, hardcoded #6c6c72 (~4:1 on the
	//                  near-black base) at 11.2px, below the 12-13px band.
	//   .text-faint  — the shared faint-text utility, dark #7d7d85 (~4.3:1),
	//                  a step below its own token --text-faint (#9a9aa2, ~6.3:1).
	// Lift both onto the theme tokens (a legible dim in each theme) and pull the
	// sizes into the 12-13px band. Colour changes are neutral gray only.
	rule(".set-label",
		fontSize("0.75rem"), // 11.2px -> 12px
		color("var(--text-dim)"),
	)
	rule(".text-faint",
		color("var(--text-faint)"), // dark #7d7d85 -> #9a9aa2
	)
	// .t-caption carried a size (12px) but no colour, inheriting whatever faint
	// parent it sat under; anchor it to a legible dim so captions never fade out.
	rule(".t-caption",
		fontSize("var(--type-12)"), // mid-band, landed on the type scale (task #32)
		color("var(--text-dim)"),
	)
}
