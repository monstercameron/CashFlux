// Package aiprompt assembles the Insights agent's system prompt (the MCP "prompts"
// half of C89): it stitches the house rules, the bounded financial-context block
// (from internal/aicontext), and a manifest of the available tools (from the C82
// agent.Registry) into one string for the model.
//
// The house rules enforce the determinism/explainability rule — the model narrates
// figures the engine computed and calls tools for exact values, rather than
// inventing numbers. Pure Go; unit-tested natively.
package aiprompt

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/agent"
)

// houseRules are the standing system instructions for the assistant.
const houseRules = `You are the CashFlux assistant — help the user understand and act on their money.
Rules:
- Narrate the figures the app computed; never invent numbers. If you need an exact value, call a tool.
- Every amount you state must come from the context below or a tool result.
- Be concise and use plain, friendly English — no jargon.
- For affordability or projections, use the affordability tool (it does real math) and explain its result.`

// System assembles the system prompt from a rendered financial-context block (e.g.
// aicontext.Context.Prompt()) and the tools available to the agent. The context and
// tool sections are included only when non-empty, so a no-context / no-tools call
// still yields the house rules.
func System(contextBlock string, tools []agent.ToolSpec) string {
	var b strings.Builder
	b.WriteString(houseRules)

	if strings.TrimSpace(contextBlock) != "" {
		b.WriteString("\n\n")
		b.WriteString(strings.TrimRight(contextBlock, "\n"))
	}

	if len(tools) > 0 {
		b.WriteString("\n\n## Tools\n")
		b.WriteString("Call a tool to fetch or compute exact figures instead of guessing. Available tools:\n")
		for _, t := range tools {
			b.WriteString("- " + t.Name)
			if strings.TrimSpace(t.Description) != "" {
				b.WriteString(": " + t.Description)
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}
