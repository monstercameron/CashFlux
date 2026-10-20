// SPDX-License-Identifier: MIT

package importdraft

import (
	"github.com/monstercameron/CashFlux/internal/catsuggest"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/staging"
)

// SuggestLocal applies user rules, 90%-similar categorized history, then the
// offline merchant dictionary without sending any data. Existing categories,
// stable IDs and other row fields are preserved.
func SuggestLocal(rows []extract.Row, userRules []rules.Rule, categories []domain.Category, history []domain.Transaction, decimals int) []extract.Row {
	updated := append([]extract.Row(nil), rows...)
	matchedHistory := newCategoryHistoryIndex(history, categories)
	for i, row := range updated {
		if row.Category != "" {
			continue
		}
		minor, err := staging.ParseAmount(row.Amount, decimals)
		if err != nil {
			continue
		}
		ruleCategory := rules.Category(userRules, row.Description, row.Description)
		if ruleCategory == "" {
			matches, _ := matchedHistory.match(row.Description, minor > 0)
			if len(matches) > 1 {
				continue // conflicting past choices need a person's review
			}
			if len(matches) == 1 {
				updated[i].Category = matchedHistory.names[matches[0].categoryID]
				continue
			}
		}
		suggestion, ok := catsuggest.Resolve(catsuggest.Input{
			Desc: row.Description, AmountMinor: minor, Categories: categories,
			RuleCategoryID: ruleCategory,
		})
		if !ok {
			continue
		}
		for _, cat := range categories {
			if cat.ID == suggestion.CategoryID {
				updated[i].Category = cat.Name
				break
			}
		}
	}
	return updated
}

// Suggest applies all local suggestions as one immutable draft change.
func (m *Model) Suggest(userRules []rules.Rule, categories []domain.Category, history []domain.Transaction, decimals int) {
	m.Rows = SuggestLocal(m.Rows, userRules, categories, history, decimals)
	m.changed()
}
