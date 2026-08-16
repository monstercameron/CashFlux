// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "testing"

func TestActionHistoryRecordsInOrderAndDropsTheLastOnUndo(t *testing.T) {
	assistantActions = nil
	NoteAssistantAction("add_transaction")
	NoteAssistantAction("create_category")
	got := AssistantActions()
	if len(got) != 2 || got[0].Tool != "add_transaction" || got[1].Tool != "create_category" {
		t.Fatalf("history = %+v, want the two actions in the order they happened", got)
	}
	last, ok := LastAssistantAction()
	if !ok || last.Tool != "create_category" {
		t.Fatalf("last = %+v/%v", last, ok)
	}
	DropLastAssistantAction()
	if got := AssistantActions(); len(got) != 1 || got[0].Tool != "add_transaction" {
		t.Fatalf("after undo, history = %+v — an undone change must not stay listed", got)
	}
}

func TestActionHistoryIgnoresAnEmptyToolName(t *testing.T) {
	assistantActions = nil
	NoteAssistantAction("")
	if got := AssistantActions(); len(got) != 0 {
		t.Fatalf("history = %+v, want nothing recorded for a nameless tool", got)
	}
}

func TestActionHistoryIsBounded(t *testing.T) {
	assistantActions = nil
	for i := 0; i < maxAssistantActions+50; i++ {
		NoteAssistantAction("add_transaction")
	}
	if got := len(AssistantActions()); got != maxAssistantActions {
		t.Fatalf("history length = %d, want it capped at %d", got, maxAssistantActions)
	}
}

func TestDropOnAnEmptyHistoryIsSafe(t *testing.T) {
	assistantActions = nil
	DropLastAssistantAction()
	if got := AssistantActions(); len(got) != 0 {
		t.Fatalf("history = %+v", got)
	}
}

func TestBudgetIsPerConversation(t *testing.T) {
	conversationLimits = map[string]conversationLimit{}
	SetAgentBudget("chat-a", 50000)
	if got := AgentBudget("chat-a"); got != 50000 {
		t.Fatalf("chat-a budget = %d", got)
	}
	if got := AgentBudget("chat-b"); got != 0 {
		t.Fatalf("chat-b inherited a cap it never set: %d", got)
	}
}

func TestBudgetRejectsANegativeCap(t *testing.T) {
	conversationLimits = map[string]conversationLimit{}
	SetAgentBudget("chat", -100)
	if got := AgentBudget("chat"); got != 0 {
		t.Fatalf("budget = %d, want a negative cap treated as no cap", got)
	}
}

func TestAnUncappedConversationIsNeverExhausted(t *testing.T) {
	conversationLimits = map[string]conversationLimit{}
	if _, capped := AgentBudgetRemaining("chat"); capped {
		t.Fatal("an unset cap reported as capped")
	}
	if AgentBudgetExhausted("chat") {
		t.Fatal("an uncapped conversation reported as out of budget")
	}
}

func TestModelIsRememberedPerConversation(t *testing.T) {
	conversationLimits = map[string]conversationLimit{}
	SetAgentModel("chat-a", "gpt-5.5")
	if got := AgentModel("chat-a"); got != "gpt-5.5" {
		t.Fatalf("model = %q", got)
	}
	if got := AgentModel("chat-b"); got != "" {
		t.Fatalf("chat-b picked up chat-a's model: %q", got)
	}
	// Setting a budget must not clear the model that was set beside it.
	SetAgentBudget("chat-a", 1000)
	if got := AgentModel("chat-a"); got != "gpt-5.5" {
		t.Fatalf("setting a budget cleared the model: %q", got)
	}
}

func TestEmptyConversationIDIsIgnored(t *testing.T) {
	conversationLimits = map[string]conversationLimit{}
	SetAgentBudget("", 1000)
	SetAgentModel("", "gpt-5.5")
	if len(conversationLimits) != 0 {
		t.Fatalf("an unsaved conversation wrote limits: %+v", conversationLimits)
	}
}
