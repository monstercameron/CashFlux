// SPDX-License-Identifier: MIT

package i18n

// qpassFKeys holds English strings added by the 2026-07-19 v1.2.7 black-box
// review lane F (notification copy + rule-name humanization, settings IA,
// goals summary/legend fixes, C416 quiet-hours + digest cadence, reports scope
// chip). Merged via init so the shared en.go is never touched by this concurrent
// lane. Later init() wins, so keys here override en.go values where noted.
var qpassFKeys = Catalog{
	// #2 — humanize the last un-mapped default alert rule (was showing the raw
	// "default-unusual" ID in the alert-settings list).
	"settings.alert.unusualCharge": "Unusual charge alerts",

	// #6 — the expanded goal card's set-aside note. The em dash + space now live
	// in the copy (overrides en.go's goals.legendNote), so the space can't be lost
	// by a CSS pseudo-element.
	"goals.legendNote": "— set-aside money stays in your accounts",

	// #5 — the Goals summary headline is the money working toward goals: saved
	// contributions PLUS reserved set-asides. "Funded so far" says so honestly, and
	// the sub-caption splits it so it never contradicts the per-card saved/set-aside
	// legend.
	"goals.fundedSoFar": "Funded so far",
	"goals.fundedSplit": "saved %s · set aside %s",

	// #7 (C416) — quiet hours + digest cadence controls in the alerts panel.
	"settings.quietHoursTitle": "Quiet hours",
	"settings.quietHoursHint":  "During these hours the feed still records alerts, but browser pop-ups stay silent. Set the same start and end to turn quiet hours off.",
	"settings.quietStart":      "From",
	"settings.quietEnd":        "Until",
	"settings.digestCadence":   "Spending digest",
	"settings.digestWeekly":    "Weekly",
	"settings.digestMonthly":   "Monthly",

	// #7 (C416) — the monthly-cadence digest copy (weekly copy already exists as
	// notify.digestTitle / notify.digestBody).
	"notify.digestTitleMonthly": "Your month in review",
	"notify.digestBodyMonthly":  "Last month: %s in, %s out.",

	// #8 — the reports Scope control's at-a-glance active-scope chip line.
	"reports.scopeActiveAria": "Active scope: %s",
	"reports.scopeAccounts":   "%d accounts",
}

func init() {
	for k, v := range qpassFKeys {
		english[k] = v
	}
}
