// SPDX-License-Identifier: MIT

package i18n

// agentTrustKeys holds English strings for the assistant "trust" surface: the
// per-conversation privacy chip (AG17), the BYO-endpoint field (AG18), and the
// transparent agent-memory editor (AG19). Kept separate from en.go (concurrent
// WIP) and merged at init, like the other en_*.go catalogs.
var agentTrustKeys = Catalog{
	// AG17 — privacy tier chip in the assistant header.
	"insights.privacyLabel":          "Privacy",
	"insights.privacyFull":           "Full detail",
	"insights.privacyAggregates":     "Aggregates only",
	"insights.privacyFullHint":       "The assistant can see your totals, KPIs, and recent transactions. Click to switch to aggregates only.",
	"insights.privacyAggregatesHint": "The assistant sees only totals and KPIs — no individual transactions or payees. Click to switch back to full detail.",
	"insights.privacyAria":           "Conversation privacy: %s. Activate to change.",

	// AG18 — BYO OpenAI-compatible endpoint (Settings → AI).
	"settings.aiBaseUrlTitle":       "API endpoint",
	"settings.aiBaseUrlPlaceholder": "https://api.openai.com/v1",
	"settings.aiBaseUrlHint":        "Point at any OpenAI-compatible endpoint — e.g. a local model (Ollama, LM Studio) or a proxy. Leave blank to use OpenAI directly.",
	"settings.aiBaseUrlLocal":       "This looks like a local endpoint — your requests stay on your machine and a real API key may not be needed.",

	// AG19 — transparent agent memory editor (Settings → AI).
	"settings.memoryTitle":          "What the assistant remembers",
	"settings.memoryHint":           "Durable facts the assistant keeps across conversations — added only when you ask it to remember something. Edit or delete any of them here.",
	"settings.memoryEmpty":          "Nothing remembered yet. Tell the assistant to remember a fact (like “I'm paid biweekly”) and it will appear here for you to review.",
	"settings.memoryAddPlaceholder": "Add something to remember…",
	"settings.memoryAdd":            "Remember",
	"settings.memoryEditAria":       "Edit remembered fact",
	"settings.memoryDelete":         "Forget",
	"settings.memoryDeleteAria":     "Forget this remembered fact",
	"settings.memoryCount":          "%d remembered",
}

func init() {
	for k, v := range agentTrustKeys {
		english[k] = v
	}
}
