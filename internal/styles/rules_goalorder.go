// SPDX-License-Identifier: MIT

package styles

// registerGoalOrder styles the Goals "Needs a plan" lead section (2026-07-19
// Watch-first ordering refinement). The section header carries the same warn
// tone as the missed-deadline header — a decision is due — so the lead section
// reads as "act on these" at a glance. Theme token only, so both light and dark
// track automatically; the cards inside are unchanged (their pace badge and
// reason line keep their own tones from registerGoalHealthTones).
func registerGoalOrder() {
	rule(".goals-needsplan-title",
		color("var(--warn, #d8a24a)"),
	)
}
