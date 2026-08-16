// SPDX-License-Identifier: MIT

package i18n

import "maps"

// vSweepKeys holds the English copy introduced by the 2026-07-03 world-class
// visual/UX sweep's remaining tickets (C340–C362). Kept in its own file and
// merged via init so it never collides with in-flight edits to en.go, mirroring
// en_uxsweep.go.
var vSweepKeys = Catalog{
	// ── C341: the one shared net-worth-change sentence ──────────────────────
	// Every surface that prints a net-worth delta renders it through
	// screens.nwChangeSub, so the window is always named and the wording never
	// varies. %d = whole percent (always positive; the arrow carries direction),
	// first %s = the amount as a magnitude, last %s = the window name.
	"nw.windowMonthToDate": "this month",
	"nw.changeUp":          "▲ %d%% (+%s) %s",
	"nw.changeDown":        "▼ %d%% (−%s) %s",
	"nw.changeUpAmount":    "▲ +%s %s",
	"nw.changeDownAmount":  "▼ −%s %s",
	"nw.changeNone":        "No change %s",
	"nw.changeUnknown":     "Change %s isn't available yet",
	// Stat label on the Reports net-worth tab. It used to read "Change this
	// period" while showing last month's step; it now names its real window.
	"nw.changeMonthLabel": "Change this month",

	// ── C342/C343: name the window over every figure that has one ───────────
	// Caption above the dashboard hero's income / spending / net / savings-rate
	// row. %s is the selected period ("Jul 2026").
	"home.statsWindow": "For %s",
	// /health factor measurement windows. The model names the span as a key;
	// these are the only place it becomes English.
	"healthx.windowTrailing3mo":   "Averaged over the last 3 full months",
	"healthx.windowCurrentPeriod": "This period so far",
	"healthx.windowAsOfToday":     "As of today",
	// Tooltip on the period chip a windowed dashboard tile always wears — the
	// mirror of the "Today" chip a current-state tile wears when paged away.
	"widget.windowBadgeTitle": "This tile's figures cover the selected period",
}

func init() {
	maps.Copy(english, vSweepKeys)
}
