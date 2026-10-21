// SPDX-License-Identifier: MIT

package screens

import (
	"slices"

	"github.com/monstercameron/CashFlux/internal/smart"
)

// cloneSmartInsights keeps cached engine results private from both renderers.
func cloneSmartInsights(in []smart.Insight) []smart.Insight {
	out := slices.Clone(in)
	for i := range out {
		item := &out[i]
		item.TitleArgs = cloneSmartArgs(item.TitleArgs)
		item.DetailArgs = cloneSmartArgs(item.DetailArgs)
		if item.Action != nil {
			action := *item.Action
			action.LabelArgs = cloneSmartArgs(action.LabelArgs)
			item.Action = &action
		}
		item.Prepared.Evidence = slices.Clone(item.Prepared.Evidence)
		for j := range item.Prepared.Evidence {
			item.Prepared.Evidence[j].LabelArgs = cloneSmartArgs(item.Prepared.Evidence[j].LabelArgs)
		}
		item.Prepared.Assumptions = slices.Clone(item.Prepared.Assumptions)
		item.Prepared.AssumptionKeys = slices.Clone(item.Prepared.AssumptionKeys)
		item.Prepared.Choices = slices.Clone(item.Prepared.Choices)
		for j := range item.Prepared.Choices {
			choice := &item.Prepared.Choices[j]
			choice.LabelArgs = cloneSmartArgs(choice.LabelArgs)
			choice.Action.LabelArgs = cloneSmartArgs(choice.Action.LabelArgs)
		}
	}
	return out
}

// Engine translation arguments are scalar today. Clone the common containers
// too, so a future detector cannot hand a renderer mutable cached arguments.
func cloneSmartArgs(in []any) []any {
	out := slices.Clone(in)
	for i, arg := range out {
		out[i] = cloneSmartArg(arg)
	}
	return out
}

func cloneSmartArg(arg any) any {
	switch value := arg.(type) {
	case []any:
		return cloneSmartArgs(value)
	case []string:
		return slices.Clone(value)
	case map[string]any:
		out := make(map[string]any, len(value))
		for key, item := range value {
			out[key] = cloneSmartArg(item)
		}
		return out
	case map[string]string:
		out := make(map[string]string, len(value))
		for key, item := range value {
			out[key] = item
		}
		return out
	default:
		return arg
	}
}
