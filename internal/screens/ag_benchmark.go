// SPDX-License-Identifier: MIT

//go:build js && wasm

// COORDINATOR: register via append(tools, agToolsBenchmark(app, base, rates)...) in buildChatTools

package screens

import (
	"encoding/json"
	"strings"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/benchmark"
	"github.com/monstercameron/CashFlux/internal/currency"
)

// agToolsBenchmark exposes web-grounded benchmarking (AG9) as an assistant tool.
// The model gathers the user's own figure (from the finance tools) and a typical
// range (from web_search, with its source), then calls this to STRUCTURE the
// answer in the required shape: the local figure, the external range with its
// source, an explicit verdict, and a stated list of assumptions — never vibes. The
// pure benchmark package computes the verdict deterministically.
func agToolsBenchmark(app *appstate.App, base string, rates currency.Rates) []chatTool {
	return []chatTool{
		{
			spec: ai.FunctionTool("compare_to_range",
				"Structure a 'your figure vs. typical range' benchmark answer (AG9). First get the user's actual figure (finance tools) and a typical range with its source (web_search); then call this with the amount, the range low/high, the source, and the assumptions you're making (region, coverage, household — whatever the range doesn't pin down). Returns a disciplined comparison: local figure, range with source, verdict, and the assumptions spelled out. Always pass the assumptions.",
				json.RawMessage(`{"type":"object","properties":{"category":{"type":"string","description":"what's being compared, e.g. car insurance"},"amount":{"type":"number","description":"the user's figure in the base currency's major units"},"low":{"type":"number","description":"typical range low, major units"},"high":{"type":"number","description":"typical range high, major units"},"source":{"type":"string","description":"citation for the range (URL or publication)"},"assumptions":{"type":"array","items":{"type":"string"}}},"required":["amount","low","high"]}`)),
			run: func(raw json.RawMessage) string {
				var a struct {
					Category    string   `json:"category"`
					Amount      float64  `json:"amount"`
					Low         float64  `json:"low"`
					High        float64  `json:"high"`
					Source      string   `json:"source"`
					Assumptions []string `json:"assumptions"`
				}
				if err := json.Unmarshal(raw, &a); err != nil {
					return "Couldn't read the comparison details."
				}
				cmp := benchmark.Compare(
					a.Category,
					currency.MinorFromMajor(a.Amount, base),
					currency.MinorFromMajor(a.Low, base),
					currency.MinorFromMajor(a.High, base),
					base, a.Assumptions,
				)
				return cmp.Format(strings.TrimSpace(a.Source))
			},
		},
	}
}
