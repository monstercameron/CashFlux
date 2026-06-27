// SPDX-License-Identifier: MIT

package i18n

// plansKeys holds the English strings for the Plans comparison screen (R31-plans).
// Added via init to avoid touching the user-WIP en.go file.
var plansKeys = Catalog{
	"plans.pageTitle":    "Plans",
	"plans.pageSub":      "Simple, honest pricing.",
	"plans.freeTitle":    "Free — always",
	"plans.freeTagline":  "Everything on this device, forever.",
	"plans.freeFeature1": "Full budgeting, goals, and reports",
	"plans.freeFeature2": "Planning, debt payoff, and insights",
	"plans.freeFeature3": "AI with your own OpenAI key",
	"plans.freeFeature4": "No account required · no expiry",
	"plans.freePrice":    "$0 / forever",
	"plans.cloudTitle":   "Cloud — optional add-on",
	"plans.cloudTagline": "Sync, backup, and bundled AI across devices.",
	"plans.cloudFeature1": "Everything in Free",
	"plans.cloudFeature2": "Multi-device sync (encrypted)",
	"plans.cloudFeature3": "Automatic encrypted backups",
	"plans.cloudFeature4": "Bundled AI — no separate key needed",
	"plans.cloudTrial":   "14-day free trial. Cancel anytime.",
	"plans.cloudTrust":   "Cancel or export anytime · end-to-end encrypted · the app stays free and local.",
	"plans.startTrial":   "Start free trial",
	"plans.manageSub":    "Manage subscription",
	"plans.learnCloud":   "Set up Cloud sync",
	"plans.currentPlan":  "Free · on this device",
	"plans.backToPlans":  "View plans →",
	"plans.orSelfHost":   "Prefer to self-host? Run your own CashFlux server — same app, your data, no lock-in. See Settings → Cloud.",
}

func init() {
	for k, v := range plansKeys {
		english[k] = v
	}
}
