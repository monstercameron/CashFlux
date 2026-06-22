package reports

import (
	"sort"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// RollUpByParent re-aggregates per-category spend rows into their TOP-LEVEL
// ancestor category, so a parent like "Food" shows the combined spend of its
// sub-categories (Groceries, Dining, Coffee) instead of each appearing
// separately (L28). A category with no parent keeps its own id; an unmapped id
// (e.g. uncategorized "") also keeps itself. DeltaPct/HasDelta are recomputed
// from the rolled-up Amount/Prior. Output is sorted largest spend first, ties
// broken by id for determinism.
func RollUpByParent(rows []CategorySpend, cats []domain.Category) []CategorySpend {
	top := topLevelMap(cats)
	agg := map[string]*CategorySpend{}
	order := make([]string, 0, len(rows))
	for _, r := range rows {
		root, ok := top[r.CategoryID]
		if !ok || root == "" {
			root = r.CategoryID
		}
		a := agg[root]
		if a == nil {
			a = &CategorySpend{CategoryID: root}
			agg[root] = a
			order = append(order, root)
		}
		a.Amount += r.Amount
		a.Prior += r.Prior
	}
	out := make([]CategorySpend, 0, len(order))
	for _, id := range order {
		a := agg[id]
		if a.Prior > 0 {
			a.DeltaPct = int64(float64(a.Amount-a.Prior) / float64(a.Prior) * 100)
			a.HasDelta = true
		}
		out = append(out, *a)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Amount != out[j].Amount {
			return out[i].Amount > out[j].Amount
		}
		return out[i].CategoryID < out[j].CategoryID
	})
	return out
}

// topLevelMap maps each category id to the id of its top-level ancestor (the
// first ancestor with no parent), following ParentID upward with a cycle guard.
func topLevelMap(cats []domain.Category) map[string]string {
	parent := make(map[string]string, len(cats))
	for _, c := range cats {
		if c.ParentID != c.ID {
			parent[c.ID] = c.ParentID
		}
	}
	top := make(map[string]string, len(cats))
	for _, c := range cats {
		id := c.ID
		seen := map[string]bool{id: true}
		for {
			p, ok := parent[id]
			if !ok || p == "" || seen[p] {
				break
			}
			id = p
			seen[id] = true
		}
		top[c.ID] = id
	}
	return top
}
