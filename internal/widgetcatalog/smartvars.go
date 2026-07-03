// SPDX-License-Identifier: MIT

package widgetcatalog

// GroupSmart collects the smart_* posture variables — how many opt-in Smart
// features are on, by tier — in the formula picker.
const GroupSmart Group = "Smart features"

// smartMetricMeta labels the fixed smart-layer variables for the picker, in
// the same order as engineenv.SmartVarNames.
var smartMetricMeta = []struct{ Name, Label, Doc string }{
	{"smart_features_on", "Smart features on", "How many opt-in Smart features are currently enabled."},
	{"smart_free_on", "Free features on", "Enabled Free features — they run on this device and cost nothing."},
	{"smart_ai_on", "AI features on", "Enabled AI features — each run is billed to your own key."},
}

// SmartMetrics exposes the smart-layer posture variables (engineenv.addSmartVars)
// in the formula picker under the Smart features group.
func SmartMetrics() []Metric {
	out := make([]Metric, 0, len(smartMetricMeta))
	for _, m := range smartMetricMeta {
		out = append(out, Metric{Name: m.Name, Label: m.Label, Doc: m.Doc, Group: GroupSmart})
	}
	return out
}
