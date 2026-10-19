// SPDX-License-Identifier: MIT

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// budgetListSearchVisible reports whether a budget list is long enough to show search.
func budgetListSearchVisible(count int) bool { return count >= 6 }

// filterBudgetStatuses applies the list's name/category/tag search and attention filter.
func filterBudgetStatuses(statuses []budgeting.Status, categories map[string]string, query, attention string) []budgeting.Status {
	query = strings.ToLower(strings.TrimSpace(query))
	filtered := make([]budgeting.Status, 0, len(statuses))
	for _, status := range statuses {
		if query != "" && !budgetSearchMatches(status.Budget, categories, query) {
			continue
		}
		if attention != "" && (attention != "needs-attention" || !budgeting.NeedsViewAttention(status)) {
			continue
		}
		filtered = append(filtered, status)
	}
	return filtered
}

// budgetSearchMatches reports whether the query appears in the budget name, tracked
// category names, or tracked tags.
func budgetSearchMatches(b domain.Budget, catNames map[string]string, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	haystack := strings.ToLower(b.Name)
	for _, id := range b.TrackedCategoryIDs() {
		haystack += " " + strings.ToLower(catNames[id])
	}
	for _, tag := range b.TrackedTags {
		haystack += " " + strings.ToLower(tag)
	}
	return strings.Contains(haystack, query)
}
