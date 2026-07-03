// SPDX-License-Identifier: MIT

package i18n

// assistantSurfaceKeys holds the English strings for the agent-first /assistant
// surface (the conversation-led layout + the agent intro + the side rail).
// Merged via init so this file does not touch en.go; keys here may also
// deliberately override en.go entries (var initialization runs before init).
var assistantSurfaceKeys = Catalog{
	"assistant.agentTitle":    "Your agent",
	"assistant.introTitle":    "What should we work on?",
	"assistant.introBody":     "I'm your CashFlux agent. I work with your real, on-device figures — and I never change anything without asking you first.",
	"assistant.capAskTag":     "Ask",
	"assistant.capAsk":        "Anything about your money — balances, spending, budgets, goals, whether you can afford something.",
	"assistant.capDoTag":      "Do",
	"assistant.capDo":         "Tell me to log a transaction, add an account or a to-do, record a transfer, or update a balance. Every change waits for your approval in the thread.",
	"assistant.capEstimateTag": "Estimate",
	"assistant.capEstimate":   "Things your data doesn't hold directly — taxes, rates, projections — with the calculator and web search, assumptions stated.",
	"assistant.observations":  "What I noticed",
	"assistant.conversations": "Conversations",
	"assistant.railHint":      "Chats are saved on this device.",

	// The keyless callout inside the intro — the one place the key pitch lives
	// on an empty thread.
	"assistant.keyCallout": "Right now I answer a fixed set of questions straight from your data. Add your OpenAI key in Settings to unlock the full agent — every tool, approved actions, web search.",

	// Composer, agent-voiced (overrides the en.go placeholder) — echoes all
	// three verbs the intro just taught.
	"insights.askPlaceholder": "Ask, tell me what to do, or have me estimate something…",
}

func init() {
	for k, v := range assistantSurfaceKeys {
		english[k] = v
	}
}
