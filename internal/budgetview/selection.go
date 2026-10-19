// SPDX-License-Identifier: MIT

// Package budgetview contains pure presentation and draft decisions for the
// Budgets screen. Inputs are snapshots; results never mutate those snapshots.
package budgetview

import (
	"fmt"
	"maps"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/catname"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Translate resolves a catalog key using the caller's active language.
type Translate func(string, ...any) string

// FormatMoney uses the screen's accounting format without browser dependencies.
func FormatMoney(m money.Money) string {
	return money.FormatAccounting(m.Amount, currency.Decimals(m.Currency), currency.Symbol(m.Currency))
}

// BaseCurrency applies the screen's historical empty-currency fallback.
func BaseCurrency(base string) string {
	if base == "" {
		return "USD"
	}
	return base
}

// FindBudget returns the first budget with the requested identity.
func FindBudget(budgets []domain.Budget, id string) (domain.Budget, bool) {
	for _, b := range budgets {
		if b.ID == id {
			return b, true
		}
	}
	return domain.Budget{}, false
}

// SelectionSeed returns independent checklist maps, normalizing tag keys.
func SelectionSeed(b domain.Budget) (map[string]bool, map[string]bool) {
	cats, tags := map[string]bool{}, map[string]bool{}
	for _, id := range b.TrackedCategoryIDs() {
		cats[id] = true
	}
	for _, tag := range b.TrackedTags {
		if k := normalizeTag(tag); k != "" {
			tags[k] = true
		}
	}
	return cats, tags
}

func normalizeTag(tag string) string { return strings.ToLower(strings.TrimSpace(tag)) }

// ToggleSelection copies a checklist before toggling a key. Tag checklists ignore
// blank keys and normalize case and whitespace, just as their seed does.
func ToggleSelection(picked map[string]bool, key string, tag bool) map[string]bool {
	next := make(map[string]bool, len(picked)+1)
	maps.Copy(next, picked)
	if tag {
		key = normalizeTag(key)
		if key == "" {
			return next
		}
	}
	next[key] = !next[key]
	return next
}

// SelectionCount counts enabled, nonempty keys (including stale category keys).
func SelectionCount(picked map[string]bool) int {
	n := 0
	for key, on := range picked {
		if on && key != "" {
			n++
		}
	}
	return n
}

// TrackingDraft builds the historical single-category or multi-category shape.
// Tags are sorted for deterministic persistence. False means there is no valid pick.
func TrackingDraft(b domain.Budget, cats []domain.Category, picked, tags map[string]bool) (domain.Budget, bool) {
	sel := budgeting.SelectedBudgetCategories(cats, picked, b.IsSaving())
	var selectedTags []string
	for k, on := range tags {
		if on && k != "" {
			selectedTags = append(selectedTags, k)
		}
	}
	sort.Strings(selectedTags)
	if len(sel) == 0 && len(selectedTags) == 0 {
		return b, false
	}
	switch len(sel) {
	case 0:
		b.CategoryID, b.CategoryIDs = "", nil
	case 1:
		b.CategoryID, b.CategoryIDs = sel[0], nil
	default:
		b.CategoryID, b.CategoryIDs = sel[0], sel
	}
	b.TrackedTags = selectedTags
	return b, true
}

// CategoryOption is one filtered checklist row, with its path and overlap label.
type CategoryOption struct{ ID, Name, AlsoIn string }

// CategoryOptions preserves input order and counts categories hidden by kind
// independently of the search. The first named overlapping budget wins.
func CategoryOptions(cats []domain.Category, budgets []domain.Budget, exclude, query string, saving bool) ([]CategoryOption, int) {
	other := map[string]string{}
	for _, b := range budgets {
		if b.ID == exclude {
			continue
		}
		for _, id := range b.TrackedCategoryIDs() {
			if other[id] == "" {
				other[id] = b.Name
			}
		}
	}
	q := normalizeTag(query)
	hidden := 0
	var shown []CategoryOption
	for _, c := range cats {
		if !saving && c.Kind != domain.KindExpense {
			hidden++
			continue
		}
		label := catname.Label(cats, c)
		if q != "" && !strings.Contains(strings.ToLower(label), q) {
			continue
		}
		shown = append(shown, CategoryOption{ID: c.ID, Name: label, AlsoIn: other[c.ID]})
	}
	return shown, hidden
}

// DistinctTags returns normalized, nonempty tags in lexical order.
func DistinctTags(txns []domain.Transaction) []string {
	seen := map[string]bool{}
	var out []string
	for _, txn := range txns {
		for _, tag := range txn.Tags {
			if k := normalizeTag(tag); k != "" && !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
	}
	sort.Strings(out)
	return out
}

// TagOptions combines existing and selected custom tags, filters them, and
// returns the normalized query and whether an exact match already exists.
func TagOptions(allTags []string, picked map[string]bool, query string) (shown []string, q string, exact bool) {
	seen := map[string]bool{}
	var all []string
	for _, tag := range allTags {
		if !seen[tag] {
			seen[tag] = true
			all = append(all, tag)
		}
	}
	for tag, on := range picked {
		if on && !seen[tag] {
			seen[tag] = true
			all = append(all, tag)
		}
	}
	sort.Strings(all)
	q = normalizeTag(query)
	for _, tag := range all {
		if q == "" || strings.Contains(tag, q) {
			shown = append(shown, tag)
		}
		if tag == q {
			exact = true
		}
	}
	return
}

// CountTotalLabel formats picker metadata; a nonpositive count has no label.
func CountTotalLabel(n int, total string) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf("%d \u00b7 %s", n, total)
}

// TrackingMetadata contains formatted metadata for the selected period.
type TrackingMetadata struct {
	Categories map[string]string
	TagCount   map[string]int
	TagTotal   map[string]string
}

// TrackingMeta delegates amounts and checked currency arithmetic to budgeting.
// If either endpoint is missing, both fall back to the supplied current month.
func TrackingMeta(ledger budgeting.NettedLedger, from, to, now time.Time, base string, rates currency.Rates) TrackingMetadata {
	if from.IsZero() || to.IsZero() {
		from, to = dateutil.MonthRange(now)
	}
	meta := budgeting.ComputeTrackingMetadata(ledger, from, to, base, rates)
	out := TrackingMetadata{Categories: map[string]string{}, TagCount: meta.TagCount, TagTotal: map[string]string{}}
	for id, n := range meta.CategoryCount {
		out.Categories[id] = CountTotalLabel(n, FormatMoney(money.New(meta.CategoryTotal[id], base)))
	}
	for tag, amount := range meta.TagTotal {
		out.TagTotal[tag] = FormatMoney(money.New(amount, base))
	}
	return out
}

// CheckedRowClass preserves the checklist's selected modifier.
func CheckedRowClass(checked bool) string {
	if checked {
		return "budgetcat-row is-on"
	}
	return "budgetcat-row"
}
