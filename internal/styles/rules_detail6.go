// SPDX-License-Identifier: MIT

//go:build js && wasm

package styles

// registerDetail6 emits the 2026-07-19 detail-lane-6 polish styles: the
// current-section highlight in the Annual Review's sticky index (scroll-spy
// .is-current), and a calmer resting state for the notification row action
// cluster (primary + snooze + overflow dimmed at rest, full contrast on row
// hover / focus-within). Registered LAST so it wins equal-specificity ties over
// rules_gen / rules_notif / rules_notifasst. Theme tokens only, so light and dark
// need no per-scheme overrides.
func registerDetail6() {
	// ── Annual Review: current-section highlight in the sticky index ───────────
	// The scroll-spy UseEffect toggles .is-current on the index item whose section
	// is in view; a tinted accent fill (plus an accent number + a soft dot ring)
	// keeps the reader oriented in the long document. The item already transitions
	// background/color, so the move in and out is smooth.
	rule(".rpta-idx-item.is-current",
		background("color-mix(in srgb, var(--accent) 14%, transparent)"),
		color("var(--text)"),
		fontWeight("600"),
	)
	rule(".rpta-idx-item.is-current .rpta-idx-num",
		color("var(--accent)"),
	)
	rule(".rpta-idx-item.is-current .rpta-idx-dot",
		prop("box-shadow", "0 0 0 2px color-mix(in srgb, var(--accent) 30%, transparent)"),
	)

	// ── Notifications: calm the resting row action cluster ─────────────────────
	// Primary (mark read/unread), the snooze clock, and the ••• overflow all sit
	// dimmed at rest so the alert text out-weights its controls; the whole cluster
	// returns to full contrast on row hover or when any control inside takes focus
	// (focus-within, so keyboard users get the same reveal). Scoped to
	// .notif-actions so the labeled mobile cluster (.notif-actions-m) is untouched.
	// Touch devices (no hover) keep everything visible.
	rule(".notif-primary, .notif-actions .notif-icon-btn",
		opacity("0.6"),
		transition("opacity .12s ease, border-color .12s ease, background .12s ease, color .12s ease"),
	)
	rule(".notif:hover .notif-primary, .notif:hover .notif-actions .notif-icon-btn, .notif:focus-within .notif-primary, .notif:focus-within .notif-actions .notif-icon-btn",
		opacity("1"),
	)
	ruleMedia("(hover: none)", ".notif-primary, .notif-actions .notif-icon-btn",
		opacity("1"),
	)
	// Sit the action cluster on the title's line rather than floating at the very
	// top of a tall multi-line card.
	rule(".notif-actions",
		alignSelf("flex-start"),
		marginTop("0.2rem"),
	)
}
