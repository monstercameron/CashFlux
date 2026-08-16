// SPDX-License-Identifier: MIT

//go:build js && wasm

// COORDINATOR: register via append(tools, agToolsTrust(app, base, rates)...) in buildChatTools

package screens

import (
	"encoding/json"
	"strings"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// agToolsTrust adds the transparent-memory tools (AG19): the agent can propose
// remembering a durable fact the user states ("paid biweekly", "don't suggest
// cutting eating out") and can read back what it already remembers. Capture is
// never silent — remember_fact is a mutating tool, so it runs only after the user
// approves the preview, and every stored fact is visible and editable in Settings.
//
// base and rates are unused here (the memory tools carry no money) but kept in the
// signature so registration matches the other agTools* groups.
func agToolsTrust(app *appstate.App, base string, rates currency.Rates) []chatTool {
	_ = app
	_ = base
	_ = rates
	return []chatTool{
		{
			spec: ai.FunctionTool("remember_fact",
				"Save a durable fact the user tells you to remember for future conversations (e.g. \"I'm paid biweekly\", \"don't suggest cutting eating out\"). Use this ONLY when the user asks you to remember something or clearly states a standing preference — the user approves before it's stored, and every remembered fact is visible and editable in Settings. Do not store one-off values, secrets, or anything the user didn't mean to keep.",
				json.RawMessage(`{"type":"object","properties":{"fact":{"type":"string","description":"the fact to remember, in a short first-person or descriptive sentence"}},"required":["fact"]}`)),
			mutates: true,
			preview: func(raw json.RawMessage) string {
				var a struct {
					Fact string `json:"fact"`
				}
				_ = json.Unmarshal(raw, &a)
				return "Remember: “" + strings.TrimSpace(a.Fact) + "”"
			},
			run: func(raw json.RawMessage) string {
				var a struct {
					Fact string `json:"fact"`
				}
				if err := json.Unmarshal(raw, &a); err != nil || strings.TrimSpace(a.Fact) == "" {
					return "Nothing to remember — give a fact to store."
				}
				fact := strings.TrimSpace(a.Fact)
				if uistate.RememberFact(fact) {
					return "Remembered: “" + fact + "”. You can view or edit this anytime in Settings → AI."
				}
				return "Already remembered something equivalent to “" + fact + "” — nothing to add."
			},
		},
		{
			spec: ai.FunctionTool("list_remembered_facts",
				"List the durable facts the user has asked you to remember (the agent's transparent memory). Call this to check what standing preferences or facts already apply before you propose remembering a new one or give advice.",
				json.RawMessage(`{"type":"object","properties":{}}`)),
			run: func(json.RawMessage) string {
				mem := uistate.LoadAgentMemory()
				if mem.Len() == 0 {
					return "No remembered facts yet."
				}
				var b strings.Builder
				b.WriteString("Remembered facts:\n")
				for _, f := range mem.Facts {
					b.WriteString("- " + f + "\n")
				}
				return strings.TrimRight(b.String(), "\n")
			},
		},
	}
}

// runAssistantWrite runs one approved mutating tool under the assistant's identity
// and takes an undo point for it (C389 / AG20).
//
// The changeset path (AG1) already did this, but a tool the model called directly
// did not: the write landed in the audit log attributed to the household, and the
// Activity screen could not tell a change the assistant made from one somebody
// typed. That is precisely the attribution the trust story rests on, so it is done
// here for every approved write, whichever path proposed it.
//
// The actor is cleared on the way out even if the tool panics, because a leaked
// assistant tag would mis-attribute every subsequent hand-made edit in the session
// — a worse failure than the one being fixed.
func runAssistantWrite(tool string, run func() string) string {
	auditview.SetSessionActor(auditview.ActorAssistant)
	defer auditview.SetSessionActor("")
	out := run()
	// Capture while the tag is still set, so the undo point carries it.
	auditview.CaptureNow()
	uistate.NoteAssistantAction(tool)
	return out
}
