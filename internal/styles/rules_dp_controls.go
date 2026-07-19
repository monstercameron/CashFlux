// SPDX-License-Identifier: MIT

package styles

// registerDpControls standardizes the SELECTED/ACTIVE visual language of passive
// (non-primary-action) controls by CONTROL TYPE, so a solid-green fill stays reserved
// for the ONE primary action per screen (.btn-primary is untouched). Before this pass
// selected controls spoke four different dialects (solid-green fill, green outline,
// green-tinted fill, neutral gray); here every passive selection drops the solid fill
// and speaks the treatment that matches its type:
//
//   - Navigation tab (Goals "Goals · Earmarks"): dark/transparent surface + a 2px
//     green UNDERLINE on the active tab — reads as "where you are", not a CTA.
//   - Segmented display choice (Reports "Summary · Full report"): a 1px green BORDER +
//     a subtle green tint on the selected segment.
//   - Filter chip (notif/smart triage "Needs you · Watching · History"; to-do
//     "List · Board · Calendar" + "All · Today · Overdue"): a neutral pill whose
//     selected member gets a subtle green tint + a 2px green MARKER rail (a stand-in
//     for the dot/check, since this pass may use only background/border/box-shadow/
//     color), never a solid fill.
//
// The generic uiw.Segmented primitive (.seg / .seg-btn — the Assistant "Ask · Insights
// · Smart" bar and ~15 other surfaces) already uses a NEUTRAL gray sliding pill, not a
// green passive fill, and is shared far too widely to retype from here; it is left as
// is (it does not violate the reserve-green rule).
//
// Registered LAST in Register(), so these same-specificity selectors win the cascade
// over the generated defaults (rules_gen.go), rules_reportssummary.go, rules_todocal.go,
// rules_notifyhistory.go and rules_lane6.go without editing any of them. Theme tokens
// only (var(--accent)/(--text)/(--border)); light + dark. Uses ONLY background /
// border / border-bottom / box-shadow / color.
func registerDpControls() {
	const tint = "color-mix(in srgb, var(--accent) 14%, transparent)"
	// Accent text tuned for contrast on the tint (matches lane6's AA fix); falls back
	// to the raw accent where --accent-ink is unset.
	const ink = "var(--accent-ink, var(--accent))"

	// ── Navigation tab: Goals "Goals · Earmarks" ─────────────────────────────────
	// Reserve the 2px underline slot on every tab so activating one causes no vertical
	// shift, then draw it in the accent on the active tab over a quiet surface (was a
	// solid/darkened green fill).
	rule(".goals-tab",
		borderBottom("2px solid transparent"),
	)
	rule(".goals-tab.is-active",
		background("transparent"),
		color("var(--text)"),
		borderBottom("2px solid var(--accent)"),
	)
	// lane6 + rules_gen force a darkened-green fill with white text under the light
	// theme; override to the same quiet underline treatment there too.
	rule("[data-theme=\"light\"] .goals-tab.is-active",
		background("transparent"),
		color("var(--text)"),
		borderBottom("2px solid var(--accent)"),
	)

	// ── Segmented display choice: Reports "Summary · Full report" ─────────────────
	// Reserve a 1px border box on every segment (base is border:0) so the selected
	// segment's border adds no relative shift, then mark the selected one with a green
	// border + subtle tint instead of a solid accent fill.
	rule(".rpta-mode",
		border("1px solid transparent"),
	)
	rule(".rpta-mode.is-on",
		background(tint),
		color(ink),
		border("1px solid color-mix(in srgb, var(--accent) 45%, var(--border))"),
	)

	// ── Filter chips: neutral pill, selected = subtle tint + a 2px green marker ────
	// Notifications / Smart triage: "Needs you · Watching · History".
	rule(".nhx-toggle-btn[aria-selected=\"true\"]",
		background(tint),
		color(ink),
		boxShadow("inset 2px 0 0 var(--accent)"),
	)
	// To-do view switch + quick-view lens: "List · Board · Calendar" / "All · Today ·
	// Overdue". Drop the green outline (that read as a segment) so it reads as a chip:
	// neutral pill, tint + marker on the selected member.
	rule(".tvw-btn.is-active",
		background(tint),
		color(ink),
		borderColor("transparent"),
		boxShadow("inset 2px 0 0 var(--accent)"),
	)
}
