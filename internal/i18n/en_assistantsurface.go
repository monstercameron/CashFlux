// SPDX-License-Identifier: MIT

package i18n

// assistantSurfaceKeys holds the English strings for the agent-first /assistant
// surface (the conversation-led layout + the agent intro + the side rail).
// Merged via init so this file does not touch en.go; keys here may also
// deliberately override en.go entries (var initialization runs before init).
var assistantSurfaceKeys = Catalog{
	"assistant.agentTitle":     "Your agent",
	"assistant.introTitle":     "What should we work on?",
	"assistant.introBody":      "I'm your CashFlux agent. I work with your real, on-device figures — and I never change anything without asking you first.",
	"assistant.capAskTag":      "Ask",
	"assistant.capAsk":         "Anything about your money — balances, spending, budgets, goals, whether you can afford something.",
	"assistant.capDoTag":       "Do",
	"assistant.capDo":          "Tell me to log a transaction, add an account or a to-do, record a transfer, or update a balance. Every change waits for your approval in the thread.",
	"assistant.capEstimateTag": "Estimate",
	"assistant.capEstimate":    "Things your data doesn't hold directly — taxes, rates, projections — with the calculator and web search, assumptions stated.",
	"assistant.observations":   "What I noticed",
	"assistant.conversations":  "Conversations",
	"assistant.railHint":       "Chats are saved on this device.",

	// Per-flag actions on a flagged-activity row: jump to the source, or attach it to
	// the composer as a context bubble to talk it through.
	"assistant.flagSource":      "Source",
	"assistant.flagSourceAria":  "Go to the source of this flag",
	"assistant.flagDiscuss":     "Discuss",
	"assistant.flagDiscussAria": "Attach this flag to the chat as context",

	// Attached-context bubbles on the composer (the "Discuss" action). contextLabel
	// prefixes the bubble row; contextPreamble + contextDefaultAsk build the sent
	// message when the bubbles fold in; ctxRemove labels the per-bubble remove button.
	"assistant.contextLabel":      "Context",
	"assistant.contextPreamble":   "About this flagged activity:",
	"assistant.contextDefaultAsk": "What should I do about this?",
	"assistant.ctxRemove":         "Remove context",

	// Inline model + thinking-level (reasoning effort) quick-switch in the chat header.
	"assistant.modelLabel":  "Model",
	"assistant.modelPick":   "Choose the AI model",
	"assistant.thinkLabel":  "Thinking",
	"assistant.thinkPick":   "Thinking level (reasoning effort)",
	"assistant.thinkLow":    "Low",
	"assistant.thinkMedium": "Medium",
	"assistant.thinkHigh":   "High",

	// Remediation action chips (per flagged-activity kind). Each chip's label is the
	// short button text; its …Msg is the instruction sent to the agent to START the
	// fix. Mutating fixes ask the agent to show the change for approval, never to act
	// silently. Kinds: duplicate (SMART-T2), missing (T7), spike (T6), balance (A1).
	"remedy.dupRemove":     "Remove the duplicate",
	"remedy.dupRemoveMsg":  "Remove the duplicate transaction — keep a single copy — and show me the change to approve.",
	"remedy.dupMerge":      "Merge the entries",
	"remedy.dupMergeMsg":   "Merge these duplicate entries into one transaction and show me the result to approve.",
	"remedy.dupKeep":       "Keep both",
	"remedy.dupKeepMsg":    "These are two separate real charges — keep both and dismiss the duplicate flag.",
	"remedy.dupReverse":    "Mark one reversed",
	"remedy.dupReverseMsg": "Mark one of the duplicates as reversed so it no longer counts as spending, and show me the change to approve.",

	"remedy.missAdd":       "Add it",
	"remedy.missAddMsg":    "Add the missing transaction — ask me for any details you need, then show it to approve.",
	"remedy.missPaused":    "It's paused",
	"remedy.missPausedMsg": "This recurring item is paused or cancelled — update it so it's no longer expected.",
	"remedy.missLater":     "Not due yet",
	"remedy.missLaterMsg":  "It isn't due yet — remind me about it later.",

	"remedy.spikeExplain":     "Explain the spike",
	"remedy.spikeExplainMsg":  "Break down exactly what drove this spending spike.",
	"remedy.spikeExpected":    "It's expected",
	"remedy.spikeExpectedMsg": "This spike is a known one-off — dismiss the flag.",
	"remedy.spikeGuard":       "Help me plan",
	"remedy.spikeGuardMsg":    "Help me set a budget or alert so this category doesn't surprise me again.",

	"remedy.balReconcile":    "Reconcile",
	"remedy.balReconcileMsg": "Walk me through reconciling this account's balance step by step.",
	"remedy.balUpdate":       "Update balance",
	"remedy.balUpdateMsg":    "Update this account's balance to the correct figure — ask me what it should be, then show the change to approve.",
	"remedy.balExplain":      "Explain the change",
	"remedy.balExplainMsg":   "Explain which transactions caused this balance change.",

	// The keyless callout inside the intro — the one place the key pitch lives
	// on an empty thread.
	"assistant.keyCallout": "Right now I answer a fixed set of questions straight from your data. Add your OpenAI key in Settings to unlock the full agent — every tool, approved actions, web search.",

	// Composer, agent-voiced (overrides the en.go placeholder) — echoes all
	// three verbs the intro just taught.
	"insights.askPlaceholder":        "Ask, tell me what to do, or have me estimate something…",
	"assistant.statusLive":           "Live — full agent",
	// The "add a key" call-to-action lives ONCE in the footer notice (insights.keyHint);
	// the subtitle and the composer placeholder no longer repeat it (they said the same
	// thing a third and second time on the same screen).
	"assistant.statusLocal":          "On-device answers",
	"assistant.composerHint":         "Enter to send · ↑ cycles your past questions",
	"assistant.voiceStart":           "Ask by voice",
	"assistant.voiceListening":       "Listening… speak your question",
	"assistant.speaker":              "✦ Agent",
	"insights.askPlaceholderKeyless": "Ask about your money…",
	"insights.advancedTitle":         "Model backend and the agent's editable system prompt",
}

func init() {
	for k, v := range assistantSurfaceKeys {
		english[k] = v
	}
}
