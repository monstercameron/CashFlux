// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package app — agentrunner.go wires the scheduled-question transport (AG5).
//
// appstate owns WHEN a saved question runs and what happens to its answer; it
// cannot own HOW the question is sent, because the transport is browser-only and
// appstate is pure. This file fills that slot at boot, the same way the undo and
// audit slots are filled in auditview.go.
package app

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/aiprovider"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// scheduledAgentSystemPrompt frames a scheduled question. It is deliberately
// different from the interactive assistant's prompt: nobody is present to answer a
// follow-up, so the reply has to stand alone, and there is no approval step, so it
// must not propose anything it would need permission to do.
const scheduledAgentSystemPrompt = "You are answering a question the household scheduled in advance. " +
	"Nobody is present to answer a follow-up, so give a complete answer in a few short paragraphs and " +
	"do not ask questions back. Do not propose changes to their data — this run cannot make any. " +
	"Lead with the answer, then the reasoning."

func init() {
	appstate.AgentRunner = runScheduledAgentPrompt
}

// runScheduledAgentPrompt places one scheduled question and hands the answer back.
//
// It runs WITHOUT tools. A scheduled run happens unattended, and the tool loop's
// safety rests on a person being there to approve a mutating call — so rather than
// filter the tool list and hope, the run is given a context-only conversation it
// cannot act from. The trade is a less capable answer for one that cannot surprise
// anybody, which is the right way round for something that fires on a timer.
func runScheduledAgentPrompt(prompt string, deliver func(string)) {
	app := appstate.Default
	if app == nil {
		return
	}
	settings := app.Settings()
	key := strings.TrimSpace(settings.OpenAIKey)
	if key == "" {
		return
	}
	model := strings.TrimSpace(settings.OpenAIModel)
	if model == "" {
		model = "gpt-5.4-mini"
	}
	baseURL := aiprovider.ResolveBaseURL(settings.OpenAIBaseURL, ai.DefaultBaseURL)
	msgs := []ai.Message{
		{Role: ai.RoleSystem, Content: scheduledAgentSystemPrompt},
		{Role: ai.RoleUser, Content: prompt},
	}
	ai.SendResponsesChatTools(key, baseURL, model, msgs, 0, settings.OpenAIReasoningEffort, nil,
		func(msg ai.Message, usage ai.Usage) {
			// The run is booked against the spend meter under its own name, so a
			// schedule that quietly costs money every week is visible as one.
			app.RecordAISpend("scheduled", model, usage)
			deliver(msg.Content)
			uistate.BumpDataRevision()
			uistate.RequestPersist()
		},
		func(errMsg string) {
			// The failure is left where the household will see it, rather than in a
			// console they will not open.
			uistate.PostNotice(uistate.T("workflow.agentRunFailed", errMsg), true)
		})
}
