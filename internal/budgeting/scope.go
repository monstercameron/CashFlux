// SPDX-License-Identifier: MIT

package budgeting

import (
	"sort"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Scope is everything a budget counts spend from: the categories whose charges
// land in it, and the cross-category tags it also tracks.
//
// It exists because the budget page had two answers to "what does this budget
// include?" and they disagreed (C585). The bar called EvaluateRollup, which
// counts the tracked categories PLUS their descendants PLUS anything carrying a
// tracked tag. The "Transactions" drill-through passed only TrackedCategoryIDs —
// the parents alone — so the Transportation budget could report $105.00 spent
// and open a ledger saying "No matching transactions", because every one of
// those charges sat in its Gas sub-category.
//
// One function now answers the question, and both the total and the drill-through
// read it, so they cannot drift apart again.
type Scope struct {
	// CategoryIDs are the tracked categories and every descendant of them,
	// sorted so the value is stable (it becomes a persisted filter).
	CategoryIDs []string
	// Tags are the budget's tracked tags. A charge carrying one counts in full
	// whatever its category, so a scope that omitted them would under-report.
	Tags []string
}

// IsEmpty reports whether the scope selects nothing at all — a budget with no
// tracked category and no tracked tag, which has no drill-through to offer.
func (s Scope) IsEmpty() bool { return len(s.CategoryIDs) == 0 && len(s.Tags) == 0 }

// HasDescendants reports whether the scope reaches past the budget's own tracked
// categories, so the UI can say "plus its sub-categories" only when true.
func (s Scope) HasDescendants(b domain.Budget) bool {
	tracked := map[string]bool{}
	for _, id := range b.TrackedCategoryIDs() {
		tracked[id] = true
	}
	for _, id := range s.CategoryIDs {
		if !tracked[id] {
			return true
		}
	}
	return false
}

// ScopeOf resolves a budget's spend scope. descendants is the same set the bar
// passes to EvaluateRollup — categorytree.DescendantsOfAll over the budget's
// tracked categories — so the two are the same set by construction.
func ScopeOf(b domain.Budget, descendants map[string]bool) Scope {
	seen := map[string]bool{}
	var ids []string
	add := func(id string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		ids = append(ids, id)
	}
	for _, id := range b.TrackedCategoryIDs() {
		add(id)
	}
	for id := range descendants {
		add(id)
	}
	sort.Strings(ids)
	tags := append([]string(nil), b.TrackedTags...)
	return Scope{CategoryIDs: ids, Tags: tags}
}
