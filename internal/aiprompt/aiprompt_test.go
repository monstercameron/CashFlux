package aiprompt

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/agent"
)

func TestSystemFull(t *testing.T) {
	ctx := "## Financial context (June 2026)\nNet worth $12,000.00."
	tools := []agent.ToolSpec{
		{Name: "query_transactions", Description: "count + total matching transactions"},
		{Name: "affordability", Description: "can the user afford X"},
	}
	got := System(ctx, tools)

	for _, want := range []string{
		"never invent numbers",                                     // house rules
		"## Financial context (June 2026)", "Net worth $12,000.00", // context block
		"## Tools",
		"- query_transactions: count + total matching transactions",
		"- affordability: can the user afford X",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("system prompt missing %q\n---\n%s", want, got)
		}
	}
	// House rules come first.
	if !strings.HasPrefix(got, "You are the CashFlux assistant") {
		t.Errorf("house rules should lead the prompt: %.40q", got)
	}
}

func TestSystemNoContextNoTools(t *testing.T) {
	got := System("   ", nil)
	if strings.Contains(got, "## Tools") || strings.Contains(got, "## Financial context") {
		t.Errorf("empty context/tools should yield only house rules:\n%s", got)
	}
	if !strings.Contains(got, "never invent numbers") {
		t.Error("house rules should always be present")
	}
}

func TestSystemToolWithoutDescription(t *testing.T) {
	got := System("", []agent.ToolSpec{{Name: "account_balances"}})
	if !strings.Contains(got, "- account_balances\n") {
		t.Errorf("a tool with no description should render just its name: %s", got)
	}
	if strings.Contains(got, "account_balances:") {
		t.Errorf("no colon when description is empty: %s", got)
	}
}
