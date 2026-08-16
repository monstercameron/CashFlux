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
	"healthx.windowPriorPeriod":   "Last completed period — this one has barely started",
	// Tooltip on the period chip a windowed dashboard tile always wears — the
	// mirror of the "Today" chip a current-state tile wears when paged away.
	"widget.windowBadgeTitle": "This tile's figures cover the selected period",

	// ── C340: one row for one real payment ──────────────────────────────────
	// A liability's own statement bill and the recurring flow the household
	// created to pay it are the same money. The surviving row names what it
	// absorbed so the merge reads as a merge, not as a missing bill. %s is the
	// liability account's name.
	"bills.covers":     "covers %s",
	"bills.coversHint": "This payment settles %s — its statement bill is shown here, not separately",

	// ── C344: a period too young to judge says so ───────────────────────────
	// Budget card status while barely any of the period has run. "On track" on
	// day 3 is a claim about the calendar, not about the household.
	"budgets.periodJustStarted": "Period just started",
	// The reading that IS worth something while the period is young: what last
	// period actually came to. %s = last period's spend, %s = the "$X under" /
	// "$X over" gap against this period's budget.
	"budgets.priorPeriodOutcome": "Last period: %s (%s)",

	// ── C345: a due-dated alert is stamped with its deadline ────────────────
	// The feed is rebuilt on boot, so every row read "just now" — a column of
	// identical timestamps on the one surface whose job is ranking by urgency.
	// %d = whole days past the due date.
	"notifications.overdueBy": "overdue by %d days",

	// ── C347: name why a detection isn't a subscription ─────────────────────
	// "Review" reads as "we aren't sure yet". The real answer for a recurring
	// grocery run is that it isn't billed at a set price, which is what the word
	// subscription means. %d = the largest deviation from the median charge.
	"subs.varies":     "varies",
	"subs.variesHint": "Charged a different amount each time (up to %d%% apart) — this is recurring spending, not a set-price subscription",
}

func init() {
	maps.Copy(english, vSweepKeys)
}
