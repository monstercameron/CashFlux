// SPDX-License-Identifier: MIT

package i18n

// smartSurfaceKeys holds the English strings for the redesigned Smart surface
// (the /assistant Smart tab and /smart): the agent-voiced hero and its posture
// chips. Merged via init so this file does not touch en.go.
var smartSurfaceKeys = Catalog{
	"smart.heroTitle":     "Smart features",
	"smart.heroEyebrow":   "Opt-in intelligence — everything runs on this device unless it's marked AI",
	"smart.heroLabel":     "findings worth a look",
	"smart.findingsTitle": "Findings",
	"smart.chipWatching":  "Watching",

	"smart.heroVoiceOff":      "Nothing is switched on yet — flip on a few Free features below and I'll start watching your money for you.",
	"smart.heroVoiceQuiet":    "All quiet. Everything I'm watching looks normal right now.",
	"smart.heroVoiceFindings": "I've found %d things worth a look — they're listed below.",

	"smart.chipFree":     "Free · $0",
	"smart.chipAI":       "AI · billed",
	"smart.chipFindings": "Findings",
	"smart.chipDensity":  "Density",

	"smart.density.off":        "Off",
	"smart.density.minimal":    "Minimal",
	"smart.density.standard":   "Standard",
	"smart.density.everywhere": "Everywhere",
}

func init() {
	for k, v := range smartSurfaceKeys {
		english[k] = v
	}
}
