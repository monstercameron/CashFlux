// SPDX-License-Identifier: MIT

package styles

// registerDpAlign is the 2026-07-19 "left-align operational section titles"
// consistency pass (audit #4). The dashboard's bento widget headers centered
// their section/widget titles — "Needs attention", "Monthly recap", "Assets",
// "Liabilities", "Safe to spend", "Goals at a glance" — while the other eight
// pages left-align their section titles. Centered headings slow vertical scanning
// and make the dashboard read like a presentation rather than a workspace.
//
// This overrides the generated widget-header title rule (.wh h2 / .wh h3 /
// .wh .wh-title, text-align: center) back to a left edge so titles line up with
// the content beneath them and scan as a workspace. The editorial dashboard
// greeting hero is untouched: on the chrome-hover hero the .wh header's h2 is
// display:none (the greeting lives in the hero body, not a widget title), so it
// stays as the single centered hero statement.
//
// text-align only; theme tokens elsewhere track light/dark automatically.
// Registered LAST in install.go so it wins at equal specificity.
func registerDpAlign() {
	// The widget/section title keeps its flex:1 growth from the generated rule;
	// only its horizontal alignment moves from center to the left edge.
	rule(".wh h2, .wh h3, .wh .wh-title",
		textAlign("left"),
	)
}
