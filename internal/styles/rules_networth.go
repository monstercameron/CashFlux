// SPDX-License-Identifier: MIT

package styles

// registerNetWorthSurface emits the /networth bento-surface design: the host
// grid and the liability-toned share bars. The hero/delta/takeaway chrome
// reuses the shared rpt-* rules (registerReportsSurface) and the section/chip
// chrome reuses debt-*; everything is token-based so it tracks every theme.
// Registered from Register() after the main sheet.
func registerNetWorthSurface() {
	rule(".bento.bento-networth",
		prop("grid-template-rows", "auto"),
		prop("grid-auto-rows", "auto"),
	)
	rule(".bento.bento-networth > .w",
		prop("height", "auto"),
		prop("min-height", "0"),
		prop("overflow", "visible"),
	)
	// Liability share bars read in the money-negative tone, not the accent.
	rule(".share-bar-fill.nw-bar-down",
		prop("background", "var(--down, #d8716f)"),
	)
}
