// SPDX-License-Identifier: MIT

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/smart"
)

func TestCloneSmartInsightsOwnsNestedResult(t *testing.T) {
	original := []smart.Insight{{
		Key: "original", TitleArgs: []any{map[string]any{"nested": []any{[]string{"original"}}}},
		DetailArgs: []any{"original"},
		Action:     &smart.Action{LabelArgs: []any{"original"}},
		Prepared: smart.Prepared{
			Evidence:    []smart.Evidence{{LabelArgs: []any{"original"}}},
			Assumptions: []string{"original"}, AssumptionKeys: []string{"original"},
			Choices: []smart.Choice{{LabelArgs: []any{"original"}, Action: smart.Action{LabelArgs: []any{"original"}}}},
		},
	}}
	copy := cloneSmartInsights(original)
	copy[0].Key = "changed"
	copy[0].TitleArgs[0].(map[string]any)["nested"].([]any)[0].([]string)[0] = "changed"
	copy[0].DetailArgs[0] = "changed"
	copy[0].Action.LabelArgs[0] = "changed"
	copy[0].Prepared.Evidence[0].LabelArgs[0] = "changed"
	copy[0].Prepared.Assumptions[0] = "changed"
	copy[0].Prepared.AssumptionKeys[0] = "changed"
	copy[0].Prepared.Choices[0].LabelArgs[0] = "changed"
	copy[0].Prepared.Choices[0].Action.LabelArgs[0] = "changed"
	if original[0].Key != "original" ||
		original[0].TitleArgs[0].(map[string]any)["nested"].([]any)[0].([]string)[0] != "original" ||
		original[0].DetailArgs[0] != "original" || original[0].Action.LabelArgs[0] != "original" ||
		original[0].Prepared.Evidence[0].LabelArgs[0] != "original" ||
		original[0].Prepared.Assumptions[0] != "original" || original[0].Prepared.AssumptionKeys[0] != "original" ||
		original[0].Prepared.Choices[0].LabelArgs[0] != "original" ||
		original[0].Prepared.Choices[0].Action.LabelArgs[0] != "original" {
		t.Fatal("copied insight retained mutable backing storage")
	}
}
