// SPDX-License-Identifier: MIT

package importdraft

import (
	"github.com/monstercameron/CashFlux/internal/catsuggest"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/staging"
)

// SuggestLocal applies rules and the offline merchant dictionary without sending
// any data. Existing categories, stable IDs and other row fields are preserved.
func SuggestLocal(rows []extract.Row, userRules []rules.Rule, categories []domain.Category, decimals int) []extract.Row {
	updated := append([]extract.Row(nil), rows...)
	for i, row := range updated {
		if row.Category != "" {
			continue
		}
		minor, err := staging.ParseAmount(row.Amount, decimals)
		if err != nil {
			continue
		}
		suggestion, ok := catsuggest.Resolve(catsuggest.Input{
			Desc: row.Description, AmountMinor: minor, Categories: categories,
			RuleCategoryID: rules.Category(userRules, row.Description, row.Description),
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
func (m *Model) Suggest(userRules []rules.Rule, categories []domain.Category, decimals int) {
	m.Rows = SuggestLocal(m.Rows, userRules, categories, decimals)
	m.changed()
}
