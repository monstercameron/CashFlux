// SPDX-License-Identifier: MIT

package styles

// registerDpRadius is the 2026-07-19 "one corner-radius scale" pass from the
// frontend-design consistency review (audit #3). Corner radii had drifted apart
// with no hierarchy: the home hero and studio stage were generously rounded
// (18px), bento widgets were effectively square (--radius is 0px), summary/stat
// tiles sat at 10px, and the card families each picked their own value — goal
// cards, budget cards, debt/strat/investment cards and saved-scenario/allocation
// rows all at 14px, notifications at 12px, report summary cards at ~11px. Same
// visual role, different radius. This normalizes every shared container to ONE
// scale so nesting reads as a deliberate hierarchy rather than a grab-bag:
//
//	Page section (largest wrapper) .......... 12px
//	Summary card / stat tile ................ 12px
//	Row card (list item in a section) ....... 8px
//	Input / button .......................... 8px
//	Pill / status badge ..................... 9999px (already full — untouched)
//
// A stateful left accent-edge (the 3px "at risk / over / zone" border on budget,
// goal, and report-summary cards) stays INSIDE the radius: it's a border-left on
// the same box, so the rounded corners clip it — no extra work needed, and these
// rules deliberately leave that accent property alone.
//
// border-radius ONLY — no other property is touched (controls/links/alignment/
// amounts belong to other lanes). Radius isn't themed, so these are theme-agnostic.
// Registered LAST in install.go so each wins at equal specificity over the base
// rules in rules_gen.go and the raw-block report rules; higher-specificity
// contextual overrides (e.g. compact-density cards, .bento-debt .stat) are left
// as intentional exceptions.
func registerDpRadius() {
	// --- Page sections: the largest wrappers. Bring the near-square bento widget
	// and the over-rounded hero / studio stage onto the shared 12px section radius.
	rule(".w", borderRadius("var(--radius-xl)"))
	rule(".home-hero", borderRadius("var(--radius-xl)"))
	rule(".studio-stage-wrap", borderRadius("var(--radius-xl)"))

	// --- Summary cards & stat tiles: all read as the same "panel" role, so all 12px.
	// .card is already 12px (asserted for durability); .stat was 10px; the surface
	// card families (investments, debt, strategy, studio types, report summaries)
	// ranged 10–14px. .rpta-sum-col / .rpta-sum-trend carry the left accent-edge.
	rule(".card", borderRadius("var(--radius-xl)"))
	rule(".stat", borderRadius("var(--radius-xl)"))
	rule(".inv-card", borderRadius("var(--radius-xl)"))
	rule(".inv-pool-card", borderRadius("var(--radius-xl)"))
	rule(".debt-card", borderRadius("var(--radius-xl)"))
	rule(".debt-stat", borderRadius("var(--radius-xl)"))
	rule(".strat-card", borderRadius("var(--radius-xl)"))
	rule(".studio-type-card", borderRadius("var(--radius-xl)"))
	rule(".rpta-sum-col", borderRadius("var(--radius-xl)"))
	rule(".rpta-sum-trend", borderRadius("var(--radius-xl)"))

	// --- Row cards: repeated list items that live INSIDE a section. One step down
	// from the 12px section to 8px so the nesting reads. Budget & goal category
	// cards, notifications, saved what-if scenarios, allocation destinations, the
	// attention-inbox items, and goal allocation rows were 9–14px; unify at 8px.
	rule(".bento-budgets .budget", borderRadius("var(--radius-lg)"))
	rule(".bento-goals .goal-card", borderRadius("var(--radius-lg)"))
	rule(".notif", borderRadius("var(--radius-lg)"))
	rule(".plan-scenario", borderRadius("var(--radius-lg)"))
	rule(".alloc-dest", borderRadius("var(--radius-lg)"))
	rule(".attention-item", borderRadius("var(--radius-lg)"))
	rule(".goal-alloc-row", borderRadius("var(--radius-lg)"))

	// --- Inputs & buttons: the everyday controls settle at 8px (were 6px), matching
	// the row-card step so a field or button reads as the same size family as the
	// row it sits in. Only the corner radius is set here.
	rule(".btn", borderRadius("var(--radius-lg)"))
	rule(".field", borderRadius("var(--radius-lg)"))
}
