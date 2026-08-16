// SPDX-License-Identifier: MIT

package i18n

// agentRunKeys holds English copy for scheduled assistant questions (AG5). Kept
// in its own file so it doesn't touch the concurrently-edited main catalog.
var agentRunKeys = Catalog{
	"workflows.actAgentRun":            "Ask the assistant",
	"workflows.agentPromptLabel":       "The question to ask",
	"workflows.agentPromptPlaceholder": "Summarise my week and flag anything unusual",
	// The two things that surprise people about a scheduled question: it spends
	// money, and its answer waits rather than arriving as a change.
	"workflows.agentRunHint": "Each run costs one AI call. The answer waits for you in the assistant — a scheduled run never changes your data.",
	// %s = the provider's own error.
	"workflow.agentRunFailed": "A scheduled question for the assistant didn't go through: %s",
}

func init() {
	for k, v := range agentRunKeys {
		english[k] = v
	}
}
