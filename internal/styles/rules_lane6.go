// SPDX-License-Identifier: MIT

package styles

// registerLane6Fixes holds the 2026-07-17 lane-6 remediation rules.
func registerLane6Fixes() {
	// UX-08: the flip modal's front face is decorative (aria-hidden) and rotates
	// away behind the back face, but backface-visibility alone leaves its H3 in
	// hit-testing/visibility terms "visible" — a residual second dialog title.
	// Once the 550ms flip completes, the front face becomes visibility:hidden
	// (delayed so the animation itself is untouched); reduced-motion skips the
	// delay along with the flip.
	rule(".flip-inner.flipped .flip-face:not(.flip-back)",
		prop("visibility", "hidden"),
		transition("visibility 0s .55s"),
	)
	ruleMedia("(prefers-reduced-motion: reduce)", ".flip-inner.flipped .flip-face:not(.flip-back)",
		transition("none"),
	)
}
