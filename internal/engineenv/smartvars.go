// SPDX-License-Identifier: MIT

package engineenv

// This file exposes the Smart layer's posture as engine variables: how many
// opt-in intelligence features are on, split by tier. The counts come from the
// persisted Smart settings (the wasm layer feeds them via Data.Smart), so a
// dashboard widget or formula can reference the layer's footprint — e.g. an
// "if(smart_ai_on > 0, …)" cost reminder.

// SmartCounts is the Smart-settings summary the wasm layer feeds in: enabled
// feature counts by tier (Free = on-device $0 engines, AI = billed per call).
type SmartCounts struct {
	FreeOn int
	AIOn   int
}

// SmartVarNames are the fixed smart-layer variables addSmartVars exposes.
var SmartVarNames = []string{
	"smart_features_on", // total enabled Smart features
	"smart_free_on",     // enabled Free (on-device, $0) features
	"smart_ai_on",       // enabled AI (billed per call) features
}

func init() { Names = append(Names, SmartVarNames...) }

// addSmartVars exposes the enabled-feature counts.
func addSmartVars(out map[string]float64, d Data) {
	out["smart_free_on"] = float64(d.Smart.FreeOn)
	out["smart_ai_on"] = float64(d.Smart.AIOn)
	out["smart_features_on"] = float64(d.Smart.FreeOn + d.Smart.AIOn)
}
