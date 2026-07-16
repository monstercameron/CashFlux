// SPDX-License-Identifier: MIT

package i18n

// merchantTrendKeys holds the copy for the transactions-row spending-trend chip (TX6b)
// — the surfaced merchant story. The story content itself reuses the existing
// merchantPanel.* keys. Merged via init so this file does not touch en.go.
var merchantTrendKeys = Catalog{
	// Chip aria-label / tooltip. %s = merchant name.
	"merchantTrend.label": "Spending trend for %s",
	// Shown (aria) while the merchant's stats compute.
	"merchantTrend.loading": "Loading spending trend…",
	// Fallback if there turns out to be too little history to show.
	"merchantTrend.none": "Not enough history yet.",
	// Sparkline caption. %d = number of charges plotted, %s = the latest charge amount.
	"merchantTrend.sparkMeta": "%d charges · latest %s",
}

func init() {
	for k, v := range merchantTrendKeys {
		english[k] = v
	}
}
