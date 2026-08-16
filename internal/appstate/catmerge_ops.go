// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/catmerge"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
)

// categoryRefs snapshots every collection that can hold a category reference.
//
// Categories are included only when the caller intends to retire one — a merge
// removes the source and re-homes its children onto the survivor, while
// reassign-on-delete leaves both jobs to the delete path.
func (a *App) categoryRefs(withCategories bool) catmerge.Refs {
	r := catmerge.Refs{
		Transactions: a.Transactions(),
		Budgets:      a.Budgets(),
		Goals:        a.Goals(),
		Rules:        a.Rules(),
		Recurring:    a.Recurring(),
		// The two classes that are not in the store, and so were not in the
		// original sweep (C548): the UI's learn-from-corrections tally, lent in
		// through the bridge, and widget specs saved inside custom pages.
		Tally:   loadLearnTally(),
		Widgets: a.pageWidgetSpecs(),
	}
	if withCategories {
		r.Categories = a.Categories()
	}
	return r
}

// PlanCategoryMerge reports what merging fromID into toID would move, without
// changing anything — so a confirmation can name real numbers, computed by the
// same code that will perform the move.
func (a *App) PlanCategoryMerge(fromID, toID string) catmerge.Counts {
	return catmerge.Plan(a.categoryRefs(true), fromID, toID)
}

// MergeCategories moves every reference from one category onto another and
// retires the source.
//
// This is the repair tool for duplicates, and the reason the sweep is shared:
// the old reassign-on-delete path rewrote a transaction's category and a
// budget's category and nothing else, leaving split lines, multi-category budget
// lists, sinking-fund goals, rules and recurring templates pointing at an id
// that no longer existed. Those do not fail loudly — they produce totals that
// quietly stop adding up.
func (a *App) MergeCategories(fromID, toID string) (catmerge.Counts, error) {
	if err := a.roleGuard(); err != nil {
		return catmerge.Counts{}, err
	}
	if fromID == "" || toID == "" || fromID == toID {
		return catmerge.Counts{}, fmt.Errorf("merge categories: pick two different categories")
	}
	cats := a.Categories()
	var haveFrom, haveTo bool
	for _, c := range cats {
		haveFrom = haveFrom || c.ID == fromID
		haveTo = haveTo || c.ID == toID
	}
	if !haveFrom || !haveTo {
		return catmerge.Counts{}, fmt.Errorf("merge categories: one of them no longer exists")
	}

	before := a.categoryRefs(true)
	after, counts := catmerge.Merge(before, fromID, toID)

	// Persist only what actually changed, so a merge of one transaction does not
	// rewrite the whole ledger.
	prevTxn := make(map[string]string, len(before.Transactions))
	for _, t := range before.Transactions {
		prevTxn[t.ID] = t.CategoryID + "|" + splitKey(t)
	}
	for _, t := range after.Transactions {
		if prevTxn[t.ID] == t.CategoryID+"|"+splitKey(t) {
			continue
		}
		if err := a.store.PutTransaction(t); err != nil {
			return counts, fmt.Errorf("merge categories: transaction %s: %w", t.ID, err)
		}
	}
	for i, b := range after.Budgets {
		if i < len(before.Budgets) && before.Budgets[i].CategoryID == b.CategoryID &&
			sameIDs(before.Budgets[i].CategoryIDs, b.CategoryIDs) {
			continue
		}
		if err := a.store.PutBudget(b); err != nil {
			return counts, fmt.Errorf("merge categories: budget %s: %w", b.ID, err)
		}
	}
	for i, g := range after.Goals {
		if i < len(before.Goals) && before.Goals[i].CategoryID == g.CategoryID {
			continue
		}
		if err := a.store.PutGoal(g); err != nil {
			return counts, fmt.Errorf("merge categories: goal %s: %w", g.ID, err)
		}
	}
	for i, ru := range after.Rules {
		if i < len(before.Rules) && before.Rules[i].SetCategoryID == ru.SetCategoryID {
			continue
		}
		if err := a.store.PutRule(ru); err != nil {
			return counts, fmt.Errorf("merge categories: rule %s: %w", ru.ID, err)
		}
	}
	for i, rc := range after.Recurring {
		if i < len(before.Recurring) && before.Recurring[i].CategoryID == rc.CategoryID {
			continue
		}
		if err := a.store.PutRecurring(rc); err != nil {
			return counts, fmt.Errorf("merge categories: recurring %s: %w", rc.ID, err)
		}
	}
	// Children are re-homed onto the SURVIVOR before the source is removed, so
	// the delete path's own reparenting never sees an orphan to guess about.
	for i, c := range after.Categories {
		if i < len(before.Categories) && before.Categories[i].ID == c.ID && before.Categories[i].ParentID == c.ParentID {
			continue
		}
		if err := a.store.PutCategory(c); err != nil {
			return counts, fmt.Errorf("merge categories: category %s: %w", c.ID, err)
		}
	}
	if err := a.saveSweptExtras(after); err != nil {
		return counts, fmt.Errorf("merge categories: sweep: %w", err)
	}
	if _, err := a.store.DeleteCategory(fromID); err != nil {
		return counts, fmt.Errorf("merge categories: retire %s: %w", fromID, err)
	}
	a.log.Info("merged category", "from", fromID, "into", toID, "moved", counts.Total())
	return counts, nil
}

// splitKey is a cheap fingerprint of a transaction's split categories, used to
// detect whether a merge touched any of its lines. Only the category ids matter
// here — amounts and members are never changed by a merge.
func splitKey(t domain.Transaction) string {
	if len(t.Splits) == 0 {
		return ""
	}
	var b strings.Builder
	for _, s := range t.Splits {
		b.WriteString(s.CategoryID)
		b.WriteByte(',')
	}
	return b.String()
}

// sameIDs reports whether two id lists are identical in order and content.
func sameIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// moveCategoryRefs rewrites every reference from oldID to newID WITHOUT touching
// the categories themselves, and returns how many references moved. Shared by
// reassign-on-delete and by the merge, so the two can never disagree about what
// counts as a reference.
func (a *App) moveCategoryRefs(oldID, newID string) (int, error) {
	before := a.categoryRefs(false)
	after, counts := catmerge.Merge(before, oldID, newID)

	prev := make(map[string]string, len(before.Transactions))
	for _, t := range before.Transactions {
		prev[t.ID] = t.CategoryID + "|" + splitKey(t)
	}
	for _, t := range after.Transactions {
		if prev[t.ID] == t.CategoryID+"|"+splitKey(t) {
			continue
		}
		if err := a.store.PutTransaction(t); err != nil {
			return counts.Total(), err
		}
	}
	for i, b := range after.Budgets {
		if i < len(before.Budgets) && before.Budgets[i].CategoryID == b.CategoryID &&
			sameIDs(before.Budgets[i].CategoryIDs, b.CategoryIDs) {
			continue
		}
		if err := a.store.PutBudget(b); err != nil {
			return counts.Total(), err
		}
	}
	for i, g := range after.Goals {
		if i < len(before.Goals) && before.Goals[i].CategoryID == g.CategoryID {
			continue
		}
		if err := a.store.PutGoal(g); err != nil {
			return counts.Total(), err
		}
	}
	for i, ru := range after.Rules {
		if i < len(before.Rules) && before.Rules[i].SetCategoryID == ru.SetCategoryID {
			continue
		}
		if err := a.store.PutRule(ru); err != nil {
			return counts.Total(), err
		}
	}
	for i, rc := range after.Recurring {
		if i < len(before.Recurring) && before.Recurring[i].CategoryID == rc.CategoryID {
			continue
		}
		if err := a.store.PutRecurring(rc); err != nil {
			return counts.Total(), err
		}
	}
	// Reassign-on-delete sweeps the same out-of-store classes as a merge (C548):
	// a category being deleted with its references reassigned leaves exactly the
	// same stale tally entries and chart filters behind if it does not.
	if err := a.saveSweptExtras(after); err != nil {
		return counts.Total(), err
	}
	return counts.Total(), nil
}

// MergeCategoriesIntoNew creates a category and folds every source into it,
// returning the new category's id and the combined counts (C549).
//
// Cam's ask was "merge 2 categories into a new category", and what existed only
// offered targets that already exist — so the phrasing had no path at all. It
// composes rather than needing new merge logic: create the target, then run the
// same sweep once per source. Doing it here rather than in the view keeps the
// whole thing one operation the UI cannot half-perform.
//
// The new category inherits its Kind and ParentID from the FIRST source, and
// every source must share that kind: folding an expense category into an income
// one is the data-integrity hazard the merge panel already refuses (C63).
// Sibling-name uniqueness is not re-implemented here — PutCategory enforces it
// (C536/C537), so a duplicate name fails before anything is merged.
func (a *App) MergeCategoriesIntoNew(name string, sourceIDs ...string) (string, catmerge.Counts, error) {
	if err := a.roleGuard(); err != nil {
		return "", catmerge.Counts{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", catmerge.Counts{}, fmt.Errorf("merge categories: name the new category")
	}
	if len(sourceIDs) == 0 {
		return "", catmerge.Counts{}, fmt.Errorf("merge categories: pick at least one category to merge")
	}

	byID := make(map[string]domain.Category, len(a.Categories()))
	for _, c := range a.Categories() {
		byID[c.ID] = c
	}
	seen := make(map[string]bool, len(sourceIDs))
	srcs := make([]domain.Category, 0, len(sourceIDs))
	for _, id := range sourceIDs {
		c, ok := byID[id]
		if !ok {
			return "", catmerge.Counts{}, fmt.Errorf("merge categories: one of them no longer exists")
		}
		if seen[id] {
			continue // the same category twice is one source
		}
		seen[id] = true
		srcs = append(srcs, c)
	}
	for _, c := range srcs[1:] {
		if c.Kind != srcs[0].Kind {
			return "", catmerge.Counts{}, fmt.Errorf(
				"merge categories: %q and %q are different kinds — an expense and an income category cannot merge",
				srcs[0].Name, c.Name)
		}
	}

	// Create the target FIRST, so a rejected name (duplicate sibling, invalid
	// tree position) fails before any data has moved.
	target := domain.Category{
		ID:       id.New(),
		Name:     name,
		Kind:     srcs[0].Kind,
		ParentID: srcs[0].ParentID,
		Color:    srcs[0].Color,
	}
	if err := a.PutCategory(target); err != nil {
		return "", catmerge.Counts{}, err
	}

	var total catmerge.Counts
	for _, c := range srcs {
		got, err := a.MergeCategories(c.ID, target.ID)
		if err != nil {
			// The target and any completed merges stay: they are valid on their own,
			// and silently unwinding a partial merge would be a second, larger
			// mutation performed without asking.
			return target.ID, total, fmt.Errorf("merge categories: %q: %w", c.Name, err)
		}
		total = addCounts(total, got)
	}
	a.log.Info("merged categories into a new one", "into", target.ID, "sources", len(srcs), "moved", total.Total())
	return target.ID, total, nil
}

// addCounts sums two merge tallies field by field, so a multi-source merge can
// report one honest total rather than the last source's numbers.
func addCounts(a, b catmerge.Counts) catmerge.Counts {
	return catmerge.Counts{
		Transactions: a.Transactions + b.Transactions,
		Splits:       a.Splits + b.Splits,
		Budgets:      a.Budgets + b.Budgets,
		Goals:        a.Goals + b.Goals,
		Rules:        a.Rules + b.Rules,
		Recurring:    a.Recurring + b.Recurring,
		Children:     a.Children + b.Children,
		Tally:        a.Tally + b.Tally,
		Widgets:      a.Widgets + b.Widgets,
	}
}
