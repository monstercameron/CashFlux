// SPDX-License-Identifier: MIT

package txntoolbar

import (
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

// CategoryItem carries a category's proportion and selection presentation.
type CategoryItem struct {
	Category         CategoryTotal
	Selected, Dimmed bool
}

// Breakdown holds full, bar and compact-list presentations of the same totals.
type Breakdown struct {
	All, Bar, Visible []CategoryItem
	More              int
}

// CategoryLayout calculates proportions and compact ordering without modifying
// its input. Overflow suppresses percentages instead of exposing a partial sum.
func (p Presentation) CategoryLayout(categories []CategoryTotal, selected string) Breakdown {
	total := int64(0)
	for _, c := range categories {
		if c.Minor <= 0 {
			continue
		}
		next, err := money.Add(total, c.Minor)
		if err != nil {
			total = 0
			break
		}
		total = next
	}
	decorate := func(values []CategoryTotal) []CategoryItem {
		result := make([]CategoryItem, 0, len(values))
		for _, c := range values {
			c.Share = CategoryShare(c.Minor, total)
			on := selected == "__uncategorized__" && c.ID == "" || selected != "" && selected != "__uncategorized__" && c.ID == selected
			result = append(result, CategoryItem{Category: c, Selected: on, Dimmed: selected != "" && !on})
		}
		return result
	}
	result := Breakdown{All: decorate(categories), Bar: decorate(p.CategoryTopFive(categories, selected))}
	result.Visible = append([]CategoryItem(nil), result.All...)
	if len(result.Visible) > 3 {
		for i, item := range result.Visible {
			if item.Category.ID == "__other__" && i >= 3 {
				result.Visible[2], result.Visible[i] = result.Visible[i], result.Visible[2]
				break
			}
		}
		result.More = len(result.Visible) - 3
		result.Visible = result.Visible[:3]
	}
	return result
}

// ChooseCategory toggles the legend's category while preserving all other scope.
func ChooseCategory(f txnfilter.Criteria, category string) txnfilter.Criteria {
	if category == "" {
		f.Uncategorized = !f.Uncategorized
		f.Category = ""
		return f
	}
	if f.Category == category {
		f.Category = ""
	} else {
		f.Category = category
	}
	return f
}

// ToggleFlow toggles one cashflow direction.
func ToggleFlow(f txnfilter.Criteria, flow string) txnfilter.Criteria {
	if f.Flow == flow {
		f.Flow = ""
	} else {
		f.Flow = flow
	}
	return f
}
