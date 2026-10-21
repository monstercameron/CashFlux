// SPDX-License-Identifier: MIT

package assistantloop

import (
	"errors"
	"testing"

	"github.com/monstercameron/CashFlux/internal/ai"
)

func TestBoundRequestHistoryKeepsNewestCompleteTurns(t *testing.T) {
	messages := []ai.Message{
		{Role: ai.RoleSystem, Content: "system"},
		{Role: ai.RoleUser, Content: "old question"},
		{Role: ai.RoleAssistant, Content: "old answer"},
		{Role: ai.RoleUser, Content: "recent question"},
		{Role: ai.RoleAssistant, Content: "recent answer"},
		{Role: ai.RoleUser, Content: "current question"},
	}
	got, err := BoundRequestHistory(messages, 1, 5, 5, 1<<20, 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 || got[1].Content != "recent question" || got[2].Content != "recent answer" || got[3].Content != "current question" {
		t.Fatalf("bounded messages = %#v", got)
	}
}

func TestBoundRequestHistoryObeysMessageAndByteLimits(t *testing.T) {
	messages := []ai.Message{
		{Role: ai.RoleSystem, Content: "system"},
		{Role: ai.RoleUser, Content: "prior"},
		{Role: ai.RoleAssistant, Content: "reply"},
		{Role: ai.RoleUser, Content: "now"},
	}
	got, err := BoundRequestHistory(messages, 1, 3, 3, 1<<20, 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].Content != "now" {
		t.Fatalf("message-count bounded result = %#v", got)
	}
	got, err = BoundRequestHistory(messages, 1, 3, 8, 200, 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("byte-bounded result = %#v, expected this small history to fit", got)
	}
	if _, err := BoundRequestHistory(messages, 1, 3, 8, 3, 64<<10); !errors.Is(err, ErrRequiredMessagesTooLarge) {
		t.Fatalf("required-message overflow error = %v", err)
	}
}

func TestBoundRequestHistoryPreservesCurrentToolProtocol(t *testing.T) {
	call := ai.Message{Role: ai.RoleAssistant, ToolCalls: []ai.ToolCall{
		{ID: "call-a", Type: "function", Function: ai.FunctionCall{Name: "read_a", Arguments: `{}`}},
		{ID: "call-b", Type: "function", Function: ai.FunctionCall{Name: "read_b", Arguments: `{}`}},
	}}
	messages := []ai.Message{
		{Role: ai.RoleSystem, Content: "system"},
		{Role: ai.RoleUser, Content: "prior"},
		{Role: ai.RoleAssistant, Content: "prior reply"},
		{Role: ai.RoleUser, Content: "current"},
		call,
		ai.ToolResultMessage("call-b", "read_b", "b"),
		ai.ToolResultMessage("call-a", "read_a", "a"),
	}
	got, err := BoundRequestHistory(messages, 1, 3, 5, 1<<20, 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 || got[1].Content != "current" || len(got[2].ToolCalls) != 2 || got[3].Role != ai.RoleTool || got[4].Role != ai.RoleTool {
		t.Fatalf("bounded active tool protocol = %#v", got)
	}
	messages[6] = ai.ToolResultMessage("wrong-id", "read_a", "a")
	if _, err := BoundRequestHistory(messages, 1, 3, 8, 1<<20, 64<<10); !errors.Is(err, ErrIncompleteToolProtocol) {
		t.Fatalf("orphaned current tool result error = %v", err)
	}
}

func TestBoundRequestHistoryNeverKeepsPartialPriorToolBundle(t *testing.T) {
	messages := []ai.Message{
		{Role: ai.RoleSystem, Content: "system"},
		{Role: ai.RoleUser, Content: "prior"},
		{Role: ai.RoleAssistant, ToolCalls: []ai.ToolCall{{ID: "old-call", Type: "function", Function: ai.FunctionCall{Name: "read", Arguments: `{}`}}}},
		ai.ToolResultMessage("old-call", "read", "result"),
		{Role: ai.RoleAssistant, Content: "completed prior answer"},
		{Role: ai.RoleUser, Content: "current"},
	}
	got, err := BoundRequestHistory(messages, 1, 5, 5, 1<<20, 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].Content != "current" {
		t.Fatalf("partial tool bundle was retained: %#v", got)
	}
	if _, err := BoundRequestHistory(messages, 1, 3, 8, 1<<20, 64<<10); !errors.Is(err, ErrIncompleteToolProtocol) {
		t.Fatalf("incomplete current tool bundle error = %v", err)
	}
}

func TestBoundRequestHistoryDropsOversizedPriorBundleButRejectsOversizedRequiredMessage(t *testing.T) {
	messages := []ai.Message{
		{Role: ai.RoleSystem, Content: "system"},
		{Role: ai.RoleUser, Content: "small prior"},
		{Role: ai.RoleAssistant, Content: "small reply"},
		{Role: ai.RoleUser, Content: "large prior"},
		{Role: ai.RoleAssistant, Content: string(make([]byte, 256))},
		{Role: ai.RoleUser, Content: "current"},
	}
	got, err := BoundRequestHistory(messages, 1, 5, 10, 1<<20, 200)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 || got[1].Content != "small prior" || got[2].Content != "small reply" || got[3].Content != "current" {
		t.Fatalf("oversized prior bundle was not omitted as a whole: %#v", got)
	}
	messages[len(messages)-1].Content = string(make([]byte, 256))
	if _, err := BoundRequestHistory(messages, 1, 5, 10, 1<<20, 200); !errors.Is(err, ErrMessageTooLarge) {
		t.Fatalf("oversized required current message error = %v", err)
	}
}
