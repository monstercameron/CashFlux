// SPDX-License-Identifier: MIT

package importai

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/ai"
)

// ModelCapabilities describes the request options known to work for a model family.
type ModelCapabilities struct {
	Temperature       bool
	ReasoningEffort   bool
	StructuredOutputs bool
}

// CapabilitiesFor returns the explicit, conservative import request policy for a model.
func CapabilitiesFor(model string) ModelCapabilities {
	m := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.HasPrefix(m, "gpt-5"), strings.HasPrefix(m, "o1"), strings.HasPrefix(m, "o3"), strings.HasPrefix(m, "o4"), strings.HasPrefix(m, "o5"):
		return ModelCapabilities{ReasoningEffort: true, StructuredOutputs: true}
	case strings.HasPrefix(m, "gpt-4o"), strings.HasPrefix(m, "gpt-4.1"), strings.HasPrefix(m, "gpt-4.5"):
		return ModelCapabilities{Temperature: true, StructuredOutputs: true}
	default:
		return ModelCapabilities{}
	}
}

// RequestOptions returns bounded, supported options for a selected model.
func RequestOptions(model string) ai.ChatOptions {
	c := CapabilitiesFor(model)
	opts := ai.ChatOptions{MaxCompletionTokens: 2048}
	if c.Temperature {
		opts.Temperature = 0.1
	}
	if c.ReasoningEffort {
		opts.ReasoningEffort = "low"
	}
	return opts
}
