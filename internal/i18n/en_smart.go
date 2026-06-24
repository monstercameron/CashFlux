// SPDX-License-Identifier: MIT

package i18n

// smartKeys is the SMART-series (per-page intelligence) set of English strings,
// registered at init time and kept separate from en.go (user WIP) like the other
// feature key files (en_home.go / en_enterprise.go / en_a11y.go). These cover the
// /smart hub chrome and the opt-in catalog; the insight titles and reasons
// themselves are produced by the engines in plain English and are not keyed here.
var smartKeys = Catalog{
	// Navigation + screen subtitle.
	"nav.smart":       "Smart",
	"screen.smartSub": "Optional, on-device intelligence for every page",

	// Insights section.
	"smart.insightsTitle": "Your insights",
	"smart.onboard":       "Turn on a smart feature below to start seeing optional, on-device insights here. Everything is off until you choose it, and nothing leaves your device unless you enable an AI feature.",
	"smart.allClear":      "All clear — no insights need your attention right now.",
	"smart.dismiss":       "Dismiss",
	"smart.taskAdded":     "Added to your to-dos.",

	// Manage / opt-in catalog.
	"smart.manageTitle":   "Manage smart features",
	"smart.manageHint":    "Every feature is optional and off by default. Free features run entirely on your device at no cost; AI features need an inference provider and are billed per use.",
	"smart.tierFree":      "Free",
	"smart.tierAI":        "AI",
	"smart.perUse":        "/use",
	"smart.needsProvider": "needs a provider",
}

func init() {
	for k, v := range smartKeys {
		english[k] = v
	}
}
