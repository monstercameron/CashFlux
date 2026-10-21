// SPDX-License-Identifier: MIT

package reports

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// RollUpCategorySeries folds every category's monthly spending into the same
// top-level ancestor as RollUpByParent. Unknown and uncategorized IDs retain
// their own series. Output slices are owned by the result; inputs stay immutable.
func RollUpCategorySeries(trends []CategoryTrend, categories []domain.Category) (map[string][]int64, error) {
	top := topLevelMap(categories)
	out := make(map[string][]int64, len(trends))
	for _, trend := range trends {
		root, ok := top[trend.CategoryID]
		if !ok || root == "" {
			root = trend.CategoryID
		}
		series, ok := out[root]
		if !ok {
			series = make([]int64, len(trend.Spend))
		} else if len(series) != len(trend.Spend) {
			return nil, fmt.Errorf("roll up category %q: mismatched bucket counts", trend.CategoryID)
		}
		for i, amount := range trend.Spend {
			var err error
			series[i], err = money.Add(series[i], amount)
			if err != nil {
				return nil, fmt.Errorf("roll up category %q bucket %d: %w", trend.CategoryID, i, err)
			}
		}
		out[root] = series
	}
	return out, nil
}
