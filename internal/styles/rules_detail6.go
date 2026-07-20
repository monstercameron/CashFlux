// SPDX-License-Identifier: MIT

//go:build js && wasm

package styles

// registerDetail6 emits the 2026-07-19 detail-lane-6 polish styles: the
// current-section highlight in the Annual Review's sticky index (scroll-spy
// .is-current), and a calmer resting state for the notification row action
// cluster (added below). Registered LAST so it wins equal-specificity ties over
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
}
