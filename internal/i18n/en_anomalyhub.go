// SPDX-License-Identifier: MIT

package i18n

// anomalyHubKeys covers the R25 anomaly-hub dashboard widget (always-on SMART
// detector surface). Kept separate from en.go (user WIP) following the init-merge
// pattern established by en_smart.go, en_home.go, etc.
var anomalyHubKeys = Catalog{
	// Widget title shown in the bento grid header.
	"dashboard.anomalyHubTitle": "Flagged activity",

	// Empty state — nothing was detected.
	"dashboard.anomalyHubClear": "No anomalies detected — everything looks normal.",

	// Compact hint line shown below the widget title when findings are present.
	"dashboard.anomalyHubHint": "Tap any item to review it.",

	// Drill-through link at the bottom of the widget.
	"dashboard.anomalyHubViewAll": "View full analysis →",

	// Aria label for the drill-through button.
	"dashboard.anomalyHubViewAllAria": "View full anomaly analysis on the Insights screen",
}

func init() {
	for k, v := range anomalyHubKeys {
		english[k] = v
	}
}
