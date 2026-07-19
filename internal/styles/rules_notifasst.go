// SPDX-License-Identifier: MIT

//go:build js && wasm

package styles

// registerNotifAsst emits the 2026-07-19 Notifications + Assistant/Smart UX
// refinement styles: the notification per-row primary-action model (one labeled
// primary + a ••• overflow), the split global-action bar (safe actions grouped,
// the destructive Clear all set off by a hairline and styled as danger), and the
// findings-first Smart masthead (compact header + relocated posture telemetry).
// Registered LAST so it wins equal-specificity ties against the generated rules
// and rules_notif / rules_smartsurface. Light + dark are handled via theme
// tokens (var(--danger)/--border/--text/--bg), so no per-scheme overrides needed.
func registerNotifAsst() {
	// ── Notifications: per-row primary + overflow ──────────────────────────────
	// Slightly wider gap now that the cluster is a labeled pill + one icon button
	// rather than a trio of same-weight icons.
	rule(".notif-actions",
		prop("gap", "0.35rem"),
	)
	// The ONE primary action: a quiet labeled pill (mark read/unread), always
	// legible — it no longer competes with three siblings for attention.
	rule(".notif-primary",
		prop("appearance", "none"),
		prop("font-family", "inherit"),
		prop("cursor", "pointer"),
		prop("display", "inline-flex"),
		prop("align-items", "center"),
		prop("gap", "0.3rem"),
		prop("padding", "0.3rem 0.6rem"),
		prop("border-radius", "999px"),
		prop("border", "1px solid var(--border)"),
		prop("background", "transparent"),
		prop("color", "var(--text)"),
		prop("font-size", "0.78rem"),
		prop("white-space", "nowrap"),
		prop("transition", "border-color .12s ease, background .12s ease"),
	)
	rule(".notif-primary:hover",
		prop("border-color", "var(--text-dim)"),
		prop("background", "var(--bg)"),
	)
	rule(".notif-primary:focus-visible",
		prop("outline", "2px solid var(--accent)"),
		prop("outline-offset", "2px"),
	)
	rule(".notif-primary-label",
		prop("font-weight", "500"),
	)
	// The destructive item inside the ••• overflow (dismiss) reads in the danger
	// tone so it stands apart from the neutral snooze/settings items.
	rule(".add-item-danger",
		prop("color", "var(--danger)"),
	)
	rule(".add-item-danger:hover",
		prop("background", "color-mix(in srgb, var(--danger) 12%, transparent)"),
		prop("color", "var(--danger)"),
	)

	// ── Notifications: split global-action bar ─────────────────────────────────
	// Safe bulk actions (Mark all read + Alert settings) travel as one group.
	rule(".notif-summary-safe",
		prop("display", "flex"),
		prop("align-items", "center"),
		prop("gap", "0.75rem"),
		prop("flex-wrap", "wrap"),
	)
	// A hairline that sets the destructive Clear all apart from the safe group, so
	// a wipe is never adjacent to a harmless action.
	rule(".notif-summary-divider",
		prop("width", "1px"),
		prop("align-self", "stretch"),
		prop("min-height", "1.1rem"),
		prop("background", "var(--border)"),
		prop("margin", "0 0.1rem"),
	)
	// Clear all wears a danger tone at rest (not just on hover) so it never reads as
	// just another neutral chip.
	rule(".notif-clear-danger",
		prop("color", "var(--danger)"),
		prop("border-color", "color-mix(in srgb, var(--danger) 40%, var(--border))"),
	)
	rule(".notif-clear-danger:hover",
		prop("color", "#fff"),
		prop("background", "var(--danger)"),
		prop("border-color", "var(--danger)"),
	)

	// ── Smart: findings-first compact masthead ─────────────────────────────────
	// The big count sits inline with its kicker/label; posture telemetry and the
	// on-device promise moved into the manage block below the feed.
	rule(".smt-masthead-compact",
		prop("padding-bottom", "1rem"),
		prop("gap", "0.4rem"),
	)
	rule(".smt-masthead-compact .smt-headline",
		prop("align-items", "center"),
		prop("gap", "0.85rem"),
		prop("margin-top", "0"),
	)
	rule(".smt-masthead-compact .smt-count",
		prop("font-size", "2.6rem"),
	)
	rule(".smt-headline-text",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.1rem"),
	)
	rule(".smt-masthead-compact .smt-voice",
		prop("font-size", "1.05rem"),
		prop("margin-top", "0.2rem"),
	)

	// ── Smart: relocated posture telemetry (top of the manage block) ───────────
	rule(".smt-posture",
		prop("display", "flex"),
		prop("flex-direction", "column"),
		prop("gap", "0.5rem"),
	)
	rule(".smt-posture .smt-metrics",
		prop("margin-top", "0"),
	)
	rule(".smt-posture .smt-fine",
		prop("margin-top", "0.4rem"),
	)
}
