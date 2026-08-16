// SPDX-License-Identifier: MIT

// Package catmerge moves every reference to one category onto another.
//
// It exists because "which things point at a category?" had no single answer.
// The reassign-on-delete path rewrote exactly two of them — a transaction's
// CategoryID and a budget's CategoryID — and silently left behind split lines, a
// multi-category budget's CategoryIDs list, sinking-fund goals, rules that set a
// category, recurring templates, and the merged category's own children. A
// delete that leaves dangling references does not fail loudly; it produces a
// household whose totals quietly stop adding up.
//
// The sweep is a pure function over plain slices so it can be exhaustively
// tested, and so the count shown in a confirmation dialog is computed by the
// same code that performs the move. A preview that says "128 transactions" and
// an apply that moves 131 is worse than no preview at all.
//
// Pure Go, no platform dependencies; unit-tested on native Go.
package catmerge

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/rules"
)

// Refs is every collection that can hold a category reference. A caller passes
// what it has; nil slices are fine and simply contribute nothing.
type Refs struct {
	Transactions []domain.Transaction
	Budgets      []domain.Budget
	Goals        []domain.Goal
	Rules        []rules.Rule
	Categories   []domain.Category
	Recurring    []domain.Recurring
}

// Counts is how much a merge would move, broken out by what holds the
// reference so the confirmation can be specific rather than saying "some
// things".
type Counts struct {
	Transactions int // whole-transaction category
	Splits       int // individual split LINES, which may exceed Transactions
	Budgets      int // budgets tracking it, single- or multi-category
	Goals        int // sinking funds drawing from it
	Rules        int // rules that file into it
	Recurring    int // recurring templates that post into it
	Children     int // sub-categories that would be re-homed
}

// Total is every reference that would move.
func (c Counts) Total() int {
	return c.Transactions + c.Splits + c.Budgets + c.Goals + c.Rules + c.Recurring + c.Children
}

// Empty reports whether the merge would move nothing at all.
func (c Counts) Empty() bool { return c.Total() == 0 }

// Plan counts what merging fromID into toID would move, without changing
// anything. Merging a category into itself moves nothing.
func Plan(r Refs, fromID, toID string) Counts {
	var c Counts
	if fromID == "" || fromID == toID {
		return c
	}
	for _, t := range r.Transactions {
		if t.CategoryID == fromID {
			c.Transactions++
		}
		for _, s := range t.Splits {
			if s.CategoryID == fromID {
				c.Splits++
			}
		}
	}
	for _, b := range r.Budgets {
		if budgetRefs(b, fromID) {
			c.Budgets++
		}
	}
	for _, g := range r.Goals {
		if g.CategoryID == fromID {
			c.Goals++
		}
	}
	for _, ru := range r.Rules {
		if ru.SetCategoryID == fromID {
			c.Rules++
		}
	}
	for _, rc := range r.Recurring {
		if rc.CategoryID == fromID {
			c.Recurring++
		}
	}
	for _, cat := range r.Categories {
		if cat.ParentID == fromID {
			c.Children++
		}
	}
	return c
}

// budgetRefs reports whether a budget points at the category through either
// shape: the single CategoryID or the multi-category CategoryIDs list.
func budgetRefs(b domain.Budget, id string) bool {
	if b.CategoryID == id {
		return true
	}
	for _, cid := range b.CategoryIDs {
		if cid == id {
			return true
		}
	}
	return false
}

// Merge returns copies of every collection with references to fromID moved onto
// toID, plus the counts of what moved. The merged category itself is REMOVED
// from Categories; its children are re-homed onto toID rather than onto its
// parent, because a merge says "these are the same thing" and the children
// belong to the thing that survives.
//
// The input slices are never mutated. Merging into itself, or from an empty id,
// returns the input unchanged.
func Merge(r Refs, fromID, toID string) (Refs, Counts) {
	c := Plan(r, fromID, toID)
	if c.Empty() && !removesCategory(r, fromID, toID) {
		return r, c
	}
	out := Refs{
		Transactions: make([]domain.Transaction, 0, len(r.Transactions)),
		Budgets:      make([]domain.Budget, 0, len(r.Budgets)),
		Goals:        make([]domain.Goal, 0, len(r.Goals)),
		Rules:        make([]rules.Rule, 0, len(r.Rules)),
		Categories:   make([]domain.Category, 0, len(r.Categories)),
		Recurring:    make([]domain.Recurring, 0, len(r.Recurring)),
	}
	for _, t := range r.Transactions {
		if t.CategoryID == fromID {
			t.CategoryID = toID
		}
		if len(t.Splits) > 0 {
			// A split may now name the same category twice. They are NOT merged
			// into one line: each line can carry its own member attribution, and
			// collapsing them would silently reassign whose spending it was.
			splits := make([]domain.CategorySplit, len(t.Splits))
			copy(splits, t.Splits)
			for i := range splits {
				if splits[i].CategoryID == fromID {
					splits[i].CategoryID = toID
				}
			}
			t.Splits = splits
		}
		out.Transactions = append(out.Transactions, t)
	}
	for _, b := range r.Budgets {
		if b.CategoryID == fromID {
			b.CategoryID = toID
		}
		if len(b.CategoryIDs) > 0 {
			b.CategoryIDs = mergeIDList(b.CategoryIDs, fromID, toID)
		}
		out.Budgets = append(out.Budgets, b)
	}
	for _, g := range r.Goals {
		if g.CategoryID == fromID {
			g.CategoryID = toID
		}
		out.Goals = append(out.Goals, g)
	}
	for _, ru := range r.Rules {
		if ru.SetCategoryID == fromID {
			ru.SetCategoryID = toID
		}
		out.Rules = append(out.Rules, ru)
	}
	for _, rc := range r.Recurring {
		if rc.CategoryID == fromID {
			rc.CategoryID = toID
		}
		out.Recurring = append(out.Recurring, rc)
	}
	for _, cat := range r.Categories {
		if cat.ID == fromID {
			continue // the merged category stops existing
		}
		if cat.ParentID == fromID {
			cat.ParentID = toID
		}
		out.Categories = append(out.Categories, cat)
	}
	return out, c
}

// removesCategory reports whether the merge would at least delete the source
// category, which is a change even when nothing points at it.
func removesCategory(r Refs, fromID, toID string) bool {
	if fromID == "" || fromID == toID {
		return false
	}
	for _, c := range r.Categories {
		if c.ID == fromID {
			return true
		}
	}
	return false
}

// mergeIDList rewrites fromID to toID in a tracked-category list and drops the
// duplicate that creates when the list already held toID. Order is otherwise
// preserved, so a multi-category budget's primary stays first.
func mergeIDList(ids []string, fromID, toID string) []string {
	out := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id == fromID {
			id = toID
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// Residual counts references to id remaining in r. A merge that worked leaves
// zero, and this is what proves it — a sweep that misses a collection is
// invisible until a total silently stops adding up.
func Residual(r Refs, id string) Counts {
	c := Plan(r, id, "\x00none")
	for _, cat := range r.Categories {
		if cat.ID == id {
			c.Children++ // the category itself surviving is a residual reference
		}
	}
	return c
}
