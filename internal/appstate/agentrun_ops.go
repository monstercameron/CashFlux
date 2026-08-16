// SPDX-License-Identifier: MIT

// Package appstate — agentrun_ops.go runs a scheduled question through the
// assistant (AG5).
//
// "Every Friday, summarise my week and flag anything weird" was blocked on two
// things: the workflow action model had no way to express it, and the scheduled
// executor had no way to reach the chat loop. The action exists now; this file is
// the executor half.
//
// The design decision that matters: a scheduled run produces a CONVERSATION, not
// an applied change. The run happens while nobody is watching, and an unattended
// agent that can act on its own conclusions is a different product with a
// different risk profile. So the question is asked and its answer is waiting the
// next time the household opens the assistant — which is also when they are in a
// position to judge it.
package appstate

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
)

// AgentRunner places one scheduled question and delivers the answer. It is a slot
// rather than a direct call because the chat transport lives in the wasm layer,
// which appstate cannot import. The default is nil, which means "queue the
// question for the next time the assistant is opened" — the honest fallback for a
// headless environment (a test, a server-side run) rather than a silent no-op.
var AgentRunner func(prompt string, deliver func(answer string))

// ScheduledAgentPrompt is one question waiting to be asked, or asked and waiting
// to be read.
type ScheduledAgentPrompt struct {
	// ID identifies the conversation this run created.
	ID string
	// Prompt is the question, already expanded against live figures.
	Prompt string
	// At is when the schedule fired.
	At time.Time
}

// RunScheduledAgentPrompt asks a saved question and leaves the exchange as a
// conversation.
//
// It returns whether the question could be placed at all. A household with no AI
// provider configured gets a notice rather than a silent skip: a schedule that
// quietly does nothing every Friday is worse than one that says why, because the
// user set it up expecting something and has no way to discover it never ran.
func (a *App) RunScheduledAgentPrompt(prompt string) bool {
	if a == nil {
		return false
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return false
	}
	if !a.hasAIProvider() {
		a.log.Info("scheduled assistant run skipped (no provider configured)", "prompt", prompt)
		if a.Notifier != nil {
			a.Notifier("A scheduled question for the assistant was skipped: no AI key is set up.")
		}
		return false
	}

	// The conversation is created UP FRONT with the question in it, so the run is
	// visible even if the answer never arrives (the network failed, the tab
	// closed mid-flight). A conversation containing only a question is a legible
	// state; a run that left no trace at all is not.
	convID := id.New()
	now := a.clock()
	conv := domain.Conversation{
		ID:        convID,
		Title:     scheduledConversationTitle(prompt, now),
		Named:     true,
		CreatedAt: now,
		UpdatedAt: now,
		Messages: []domain.ChatMessage{
			{ID: id.New(), Role: "user", Text: prompt, CreatedAt: now},
		},
	}
	if err := a.PutConversation(conv); err != nil {
		a.logErr("scheduledAgentConversation", err)
		return false
	}
	if AgentRunner == nil {
		// No transport wired (a headless run): the question is saved and will be
		// answered when the assistant is next opened.
		return true
	}
	AgentRunner(prompt, func(answer string) { a.appendScheduledAnswer(convID, answer) })
	return true
}

// appendScheduledAnswer files the answer against the conversation the run created.
func (a *App) appendScheduledAnswer(convID, answer string) {
	if strings.TrimSpace(answer) == "" {
		return
	}
	for _, c := range a.Conversations() {
		if c.ID != convID {
			continue
		}
		c.Messages = append(c.Messages, domain.ChatMessage{
			ID: id.New(), Role: "assistant", Text: answer, CreatedAt: a.clock(),
		})
		c.UpdatedAt = a.clock()
		if err := a.PutConversation(c); err != nil {
			a.logErr("scheduledAgentAnswer", err)
		}
		return
	}
}

// hasAIProvider reports whether anything is configured that could answer: a direct
// key, or a backend the app can proxy through.
func (a *App) hasAIProvider() bool {
	s := a.Settings()
	return strings.TrimSpace(s.OpenAIKey) != ""
}

// scheduledConversationTitle names the conversation a scheduled run creates, so
// the list of chats reads as a history of them rather than as a pile of identical
// entries. The date does the disambiguating; the question does the describing.
func scheduledConversationTitle(prompt string, at time.Time) string {
	short := strings.TrimSpace(prompt)
	if r := []rune(short); len(r) > 40 {
		short = strings.TrimSpace(string(r[:40])) + "…"
	}
	return short + " · " + at.Format("2 Jan")
}
