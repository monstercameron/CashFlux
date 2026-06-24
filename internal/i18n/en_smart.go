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

	// AI feature controls.
	"smart.aiTitle":             "Ask & analyze",
	"smart.aiNeedsProvider":     "These AI features need an inference provider. Add an OpenAI key (or connect the hosted backend) in Settings to use them.",
	"smart.aiCostPrefix":        "about",
	"smart.askTitle":            "Ask about your accounts",
	"smart.askPlaceholder":      "e.g. Which account grew the most this year?",
	"smart.ask":                 "Ask",
	"smart.asking":              "Asking…",
	"smart.outlookTitle":        "Summarize my outlook",
	"smart.outlookBtn":          "Summarize",
	"smart.healthTitle":         "Explain my account health",
	"smart.healthBtn":           "Explain",
	"smart.goalTitle":           "Draft a goal from a wish",
	"smart.goalPlaceholder":     "e.g. save for a $6k Japan trip next spring",
	"smart.scenarioTitle":       "Plan a what-if scenario",
	"smart.scenarioPlaceholder": "e.g. what if I get a $500/mo raise and pay $200 extra on my card?",
	"smart.allocTitle":          "Allocate in plain English",
	"smart.allocPlaceholder":    "e.g. put most toward the credit card but keep $1k liquid",
	"smart.overlapTitle":        "Find overlapping subscriptions",
	"smart.overlapBtn":          "Check for overlaps",
	"smart.todoTitle":           "Add a to-do in plain English",
	"smart.todoPlaceholder":     "e.g. move $200 to savings next Friday",
}

func init() {
	for k, v := range smartKeys {
		english[k] = v
	}
}
