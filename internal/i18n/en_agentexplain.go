// SPDX-License-Identifier: MIT

package i18n

// agentExplainKeys holds the English strings for the explain-anything affordance
// (AG7) — the small "Explain" chip that opens the assistant pre-seeded with a
// figure's derivation. Merged via init so this file does not touch en.go.
var agentExplainKeys = Catalog{
	"explain.label": "Explain",
	"explain.aria":  "Explain how %s is calculated",
}

func init() {
	for k, v := range agentExplainKeys {
		english[k] = v
	}
}
