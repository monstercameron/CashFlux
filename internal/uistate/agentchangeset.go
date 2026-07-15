// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package uistate — agentchangeset.go holds the shared UI state for AG1
// (changeset review) and AG20 (cumulative session receipt). The pending
// changeset atom carries the agent's proposed multi-step plan from the chat tool
// loop to the review card; the per-conversation Tally accumulates what the agent
// actually applied and what it cost, for the "this chat: …" receipt.
//
// The chat tool loop (fed by the coordinator) calls SetPendingChangeset to raise
// a proposal and AddAgentActions/AddAgentCost to grow the running receipt. The
// review-card and receipt components read the atoms during render.
package uistate

import (
	"github.com/monstercameron/CashFlux/internal/agentreceipt"
	"github.com/monstercameron/CashFlux/internal/changeset"
	"github.com/monstercameron/GoWebComponents/v4/state"
)

const (
	pendingChangesetAtomID = "agent:pendingChangeset"
	agentTallyRevAtomID    = "agent:tallyRevision"
)

// PendingChangeset carries a proposed changeset from the tool loop to the review
// card. Nonce increments on every proposal so the atom value always changes
// (forcing a re-render) and the card can key its local state off it. A zero
// Nonce with a nil Set means "nothing pending".
type PendingChangeset struct {
	// ConversationID ties the proposal (and its eventual receipt) to a chat.
	ConversationID string
	// Set is the proposed changeset (pointer so the review card toggles its ops
	// in place). Nil when nothing is pending.
	Set *changeset.Changeset
	// Nonce disambiguates successive proposals and drives the card's remount key.
	Nonce int
}

// UsePendingChangeset returns the shared atom holding the current agent proposal.
// The review-card host reads it during render.
func UsePendingChangeset() state.Atom[PendingChangeset] {
	a := state.UseAtom(pendingChangesetAtomID, PendingChangeset{})
	capturedPendingCs = a
	pendingCsCaptured = true
	return a
}

var (
	capturedPendingCs state.Atom[PendingChangeset]
	pendingCsCaptured bool
	pendingCsNonce    int
)

// SetPendingChangeset raises a new agent proposal for review, from outside a
// component render (the chat tool loop). No-op until the host has rendered once.
func SetPendingChangeset(conversationID string, cs *changeset.Changeset) {
	if !pendingCsCaptured {
		return
	}
	pendingCsNonce++
	capturedPendingCs.Set(PendingChangeset{ConversationID: conversationID, Set: cs, Nonce: pendingCsNonce})
}

// ClearPendingChangeset dismisses the current proposal (e.g. after apply or a
// "not now"). Safe outside render.
func ClearPendingChangeset() {
	if !pendingCsCaptured {
		return
	}
	capturedPendingCs.Set(PendingChangeset{})
}

// ─── AG20 per-conversation cumulative receipt ───────────────────────────────

// agentTallies holds each conversation's running receipt. Keyed by conversation
// ID. Access is single-goroutine (UI/tool loop); a revision atom bump triggers
// re-render of the receipt component after a mutation.
var agentTallies = map[string]*agentreceipt.Tally{}

func tallyFor(conversationID string) *agentreceipt.Tally {
	t := agentTallies[conversationID]
	if t == nil {
		t = agentreceipt.NewTally()
		agentTallies[conversationID] = t
	}
	return t
}

// AddAgentActions records applied op Kinds (changeset.Receipt.Kinds) into the
// conversation's running receipt and re-renders the receipt component.
func AddAgentActions(conversationID string, kinds []string) {
	if conversationID == "" || len(kinds) == 0 {
		return
	}
	tallyFor(conversationID).AddKinds(kinds)
	bumpAgentTally()
}

// AddAgentCost accumulates a turn's token usage and (when known) dollar cost into
// the conversation's running receipt.
func AddAgentCost(conversationID string, tokens int, costUSD float64, hasCost bool) {
	if conversationID == "" {
		return
	}
	tallyFor(conversationID).AddCost(tokens, costUSD, hasCost)
	bumpAgentTally()
}

// AgentReceiptSummary returns the one-line cumulative receipt for a conversation
// ("This chat: …"), or "" when the agent has made no changes and spent nothing.
func AgentReceiptSummary(conversationID string) string {
	t := agentTallies[conversationID]
	if t == nil {
		return ""
	}
	return t.Summary()
}

// AgentReceiptActionCount returns how many agent operations have been applied in
// a conversation (drives whether to show the receipt at all).
func AgentReceiptActionCount(conversationID string) int {
	t := agentTallies[conversationID]
	if t == nil {
		return 0
	}
	return t.TotalActions()
}

// UseAgentTallyRevision returns the atom bumped whenever any conversation's tally
// changes, so the receipt component re-renders. Read it in the component.
func UseAgentTallyRevision() state.Atom[int] {
	a := state.UseAtom(agentTallyRevAtomID, 0)
	capturedAgentTallyRev = a
	agentTallyRevCaptured = true
	return a
}

var (
	capturedAgentTallyRev state.Atom[int]
	agentTallyRevCaptured bool
)

func bumpAgentTally() {
	if agentTallyRevCaptured {
		capturedAgentTallyRev.Set(capturedAgentTallyRev.Get() + 1)
	}
}
