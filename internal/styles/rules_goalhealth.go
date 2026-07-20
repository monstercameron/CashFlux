// SPDX-License-Identifier: MIT

package styles

// registerGoalHealthTones styles the two honest pace states added so the Goals card
// badge agrees with the Smart assistant: "Watch" (the deadline needs more than a fair
// share of free cash — a stretch) reads amber like the due-soon state, and "At risk"
// (unreachable even with all free cash) reads red like an overdue goal. The card gets a
// matching red inset for At risk so its tone and its badge never disagree. Theme tokens
// only, so both light and dark track automatically.
func registerGoalHealthTones() {
	// Watch badge — amber, matching .pace-soon so the two "keep an eye on it" states read alike.
	rule(".pace-watch",
		background("rgba(245,158,11,0.18)"),
		color("#d98c00"),
	)
	rule("[data-theme=\"light\"] .pace-watch",
		color("#b45309"),
	)
	// At-risk badge — danger, matching .pace-overdue: the deadline can't be met at the
	// available pace, the strongest pace warning short of already-overdue.
	rule(".pace-atrisk",
		background("rgba(216,113,111,0.18)"),
		color("var(--danger)"),
	)
	// At-risk card tint — a red inset + faint wash, mirroring the overdue card, so the
	// whole row reads at risk at a glance, not just the badge.
	rule(".bento-goals .goal-card.is-atrisk",
		boxShadow("inset 5px 0 0 var(--danger)"),
		background("color-mix(in srgb, var(--danger) 8%, var(--bg-elev))"),
	)
	// The diagnostic reason line under the badge — quiet by default (it's supporting
	// detail), tinted to match the verdict for the two that warn.
	rule(".goal-pace-reason",
		fontSize("var(--type-12)"),
		lineHeight("1.35"),
		color("var(--text-dim)"),
		margin("0.15rem 0 0.1rem"),
	)
	rule(".goal-pace-reason-watch",
		color("#b45309"),
	)
	rule("[data-theme=\"dark\"] .goal-pace-reason-watch",
		color("#d98c00"),
	)
	rule(".goal-pace-reason-atrisk",
		color("var(--danger)"),
	)
}
