// SPDX-License-Identifier: MIT

package importdraft

import (
	"encoding/json"
	"strconv"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/workspace"
)

// Choice is a select option without a renderer dependency.
type Choice struct {
	Value, Label string
	Selected     bool
}

// CategoryChoices retains unknown extracted categories as the final option.
func CategoryChoices(categories []domain.Category, current string, translate Translate) []Choice {
	options := []Choice{{Label: translate("transactions.noCategory"), Selected: current == ""}}
	found := false
	for _, category := range categories {
		options = append(options, Choice{Value: category.Name, Label: category.Name, Selected: current == category.Name})
		found = found || category.Name == current
	}
	if current != "" && !found {
		options = append(options, Choice{Value: current, Label: current, Selected: true})
	}
	return options
}

// ColumnChoices retains the wizard's labels and zero-based column values.
func ColumnChoices(header []string, selected string, translate Translate) []Choice {
	options := []Choice{{Value: "-1", Label: translate("documents.wizardNone"), Selected: selected == "-1" || selected == ""}}
	for i, label := range header {
		value := strconv.Itoa(i)
		if label == "" {
			label = translate("imb.column", i)
		}
		options = append(options, Choice{Value: value, Label: translate("imb.columnSuffix", label, i), Selected: selected == value})
	}
	return options
}

// ActiveWorkspace decodes the registry using the shell's existing active fallback.
func ActiveWorkspace(raw string) string {
	var registry workspace.Registry
	if json.Unmarshal([]byte(raw), &registry) != nil {
		return ""
	}
	if active, ok := registry.Active(); ok {
		return active.ID
	}
	return registry.ActiveID
}

// ImageError selects the existing type/size error key before a file is read.
func ImageError(mime string, size, limit int) string {
	if len(mime) < 6 || mime[:6] != "image/" {
		return "documents.imageTypeInvalid"
	}
	if size > limit {
		return "documents.imageTooLarge"
	}
	return ""
}
