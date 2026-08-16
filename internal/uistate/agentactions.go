// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package uistate — agentactions.go keeps the ordered list of individual changes
// the assistant made this session (C389).
//
// The AG20 tally already counts what the assistant did ("3 transactions
// categorized"). A count answers "how much" but not "which", and "which" is what
// somebody needs when one of those three was wrong. This list is the per-action
// history behind the count: what ran, in what order, and when.
//
// It is session-scoped by design. The audit log is the durable record — this is
// the short-lived index into it that the conversation itself can show, so the
// nearest thing to the change is also the fastest route back out of it.
package uistate

import (
	"time"

	"github.com/monstercameron/GoWebComponents/v5/state"
)

const agentActionsRevAtomID = "agent:actionsRevision"

// AssistantAction is one change the assistant made, in the order it happened.
type AssistantAction struct {
	// Tool is the tool that made the change ("add_transaction").
	Tool string
	// At is when it ran, for the "2 minutes ago" line in the history.
	At time.Time
}

// assistantActions is the session's ordered action list. Single-goroutine (the UI
// tool loop), like the tallies beside it.
var assistantActions []AssistantAction

// maxAssistantActions bounds the list so a very long session cannot grow it
// without limit. Older entries fall off the front; the audit log still has them.
const maxAssistantActions = 200

// NoteAssistantAction records that a tool just changed something. Called from the
// approved-write path, so a declined or read-only call never appears here.
func NoteAssistantAction(tool string) {
	if tool == "" {
		return
	}
	assistantActions = append(assistantActions, AssistantAction{Tool: tool, At: time.Now()})
	if len(assistantActions) > maxAssistantActions {
		assistantActions = append([]AssistantAction(nil), assistantActions[len(assistantActions)-maxAssistantActions:]...)
	}
	bumpAgentActions()
}

// AssistantActions returns the session's action history, newest last.
func AssistantActions() []AssistantAction {
	return append([]AssistantAction(nil), assistantActions...)
}

// LastAssistantAction returns the most recent change and whether there is one.
// Only the most recent action can honestly offer an undo button: the undo stack is
// a stack, so taking back an earlier change would mean silently reversing every
// change made after it too.
func LastAssistantAction() (AssistantAction, bool) {
	if len(assistantActions) == 0 {
		return AssistantAction{}, false
	}
	return assistantActions[len(assistantActions)-1], true
}

// DropLastAssistantAction removes the most recent action from the history, after
// it has actually been undone — a history that still lists an undone change is
// worse than no history, because the next Undo would take back the wrong thing.
func DropLastAssistantAction() {
	if len(assistantActions) == 0 {
		return
	}
	assistantActions = assistantActions[:len(assistantActions)-1]
	bumpAgentActions()
}

// ─── C390: per-conversation model and token budget ──────────────────────────

// conversationLimit is one conversation's own model choice and spending cap.
type conversationLimit struct {
	model  string
	budget int
}

// conversationLimits holds each conversation's overrides for this session. The
// durable copy lives on domain.Conversation and is restored into here when a
// conversation is opened — this map is the fast path the render reads, not the
// record. A conversation with no entry uses the household default and has no cap.
var conversationLimits = map[string]conversationLimit{}

// SetAgentModel remembers which model a conversation is using, so switching away
// and back does not silently move a long analytical thread onto whatever model was
// last picked somewhere else.
func SetAgentModel(conversationID, model string) {
	if conversationID == "" {
		return
	}
	l := conversationLimits[conversationID]
	l.model = model
	conversationLimits[conversationID] = l
	bumpAgentActions()
}

// AgentModel returns a conversation's remembered model, or "" when it has none and
// the household default should be used.
func AgentModel(conversationID string) string { return conversationLimits[conversationID].model }

// SetAgentBudget caps how many tokens a conversation may spend in total. 0 removes
// the cap. The cap is per conversation rather than global on purpose: "don't let
// THIS exploration run away" is a different decision from "cap my month".
func SetAgentBudget(conversationID string, tokens int) {
	if conversationID == "" {
		return
	}
	if tokens < 0 {
		tokens = 0
	}
	l := conversationLimits[conversationID]
	l.budget = tokens
	conversationLimits[conversationID] = l
	bumpAgentActions()
}

// AgentBudget returns a conversation's token cap, or 0 when it is uncapped.
func AgentBudget(conversationID string) int { return conversationLimits[conversationID].budget }

// AgentTokensUsed returns how many tokens a conversation has spent so far, from
// the same tally that feeds the receipt — one number, counted once.
func AgentTokensUsed(conversationID string) int {
	t := agentTallies[conversationID]
	if t == nil {
		return 0
	}
	return t.Tokens
}

// AgentBudgetRemaining returns how many tokens a conversation has left and whether
// it is capped at all. A capped conversation that has overshot reports 0, never a
// negative number.
func AgentBudgetRemaining(conversationID string) (remaining int, capped bool) {
	budget := AgentBudget(conversationID)
	if budget <= 0 {
		return 0, false
	}
	remaining = budget - AgentTokensUsed(conversationID)
	if remaining < 0 {
		remaining = 0
	}
	return remaining, true
}

// AgentBudgetExhausted reports whether a capped conversation has spent its budget.
// The check happens BEFORE a send rather than after: a cap that only notices it was
// exceeded once the money is gone is a receipt, not a budget.
func AgentBudgetExhausted(conversationID string) bool {
	remaining, capped := AgentBudgetRemaining(conversationID)
	return capped && remaining == 0
}

// UseAgentActionsRevision returns the atom bumped whenever the action history
// changes, so the history component re-renders. Read it in the component.
func UseAgentActionsRevision() state.Atom[int] {
	a := state.UseAtom(agentActionsRevAtomID, 0)
	capturedAgentActionsRev = a
	agentActionsRevCaptured = true
	return a
}

var (
	capturedAgentActionsRev state.Atom[int]
	agentActionsRevCaptured bool
)

func bumpAgentActions() {
	if agentActionsRevCaptured {
		capturedAgentActionsRev.Set(capturedAgentActionsRev.Get() + 1)
	}
}

// AgentCostSoFar returns the conversation's accumulated dollar cost and whether a
// cost is known at all.
//
// It reads the SAME tally the receipt reads, rather than re-deriving a figure from
// the token total. Re-deriving is where the composer's cost line went wrong: it fed
// the conversation's whole token count in as PROMPT tokens, which priced every
// output token at the (4–8× cheaper) input rate and understated the running spend
// by a multiple. The tally is fed the properly-split cost per turn, so there is
// exactly one correct number and both places now read it.
func AgentCostSoFar(conversationID string) (float64, bool) {
	t := agentTallies[conversationID]
	if t == nil {
		return 0, false
	}
	return t.CostUSD, t.HasCost
}
