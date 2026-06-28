// SPDX-License-Identifier: MIT

package i18n

// assistantKeys holds the English strings for the /assistant hub —
// the tabbed AI landing page that merges /insights and /smart into a single
// surface. Kept in a separate file so it can be reviewed and updated without
// touching en.go (the high-churn shared catalog) or en_smart.go.
var assistantKeys = Catalog{
	// Navigation + route spec (Label / Title / Subtitle keys for screens.Route).
	"nav.assistant":       "Assistant",
	"screen.assistantSub": "Chat, spending insights, and smart features — all in one place",

	// Segmented tab labels.
	"assistant.tabGroupLabel": "Assistant section",
	"assistant.tabAsk":        "Ask",
	"assistant.tabInsights":   "Insights",
	"assistant.tabSmart":      "Smart",
}

func init() {
	for k, v := range assistantKeys {
		english[k] = v
	}
}
