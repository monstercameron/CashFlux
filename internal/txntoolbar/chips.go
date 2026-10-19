// SPDX-License-Identifier: MIT

package txntoolbar

import (
	"github.com/monstercameron/CashFlux/internal/catname"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

// Chip is a removable filter description independent of the UI framework.
type Chip struct{ Key, Label string }

// ActiveChips omits date bounds, which belong to the period control.
func ActiveChips(active []txnfilter.ActiveFilter, label func(txnfilter.ActiveFilter) string) []Chip {
	result := make([]Chip, 0, len(active))
	for _, f := range active {
		if f.Field == txnfilter.FieldFrom || f.Field == txnfilter.FieldTo {
			continue
		}
		result = append(result, Chip{Key: string(f.Field) + "\x1f" + f.Value, Label: label(f)})
	}
	return result
}

// CategoryLabel resolves the full category path, or empty for a missing category.
func CategoryLabel(categories []domain.Category, id string) string {
	for _, category := range categories {
		if category.ID == id {
			return catname.Path(categories, category)
		}
	}
	return ""
}

// OwnerOptions puts the explicit unassigned choice before household members.
func OwnerOptions(members []domain.Member, unassigned string) []PickerOption {
	result := make([]PickerOption, 0, len(members)+1)
	result = append(result, PickerOption{Value: UnassignedOwner, Label: unassigned})
	for _, member := range members {
		result = append(result, PickerOption{Value: member.ID, Label: member.Name})
	}
	return result
}
