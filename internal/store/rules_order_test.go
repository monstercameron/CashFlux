package store

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/rules"
)

// ListRules must return rules in precedence order (Order asc, then id) so the
// "first match wins" engine honors the user's drag-to-reorder (C64).
func TestListRulesPrecedenceOrder(t *testing.T) {
	s := newStore(t)
	// Insert out of order; ids chosen so id-sort alone would NOT yield Order-sort.
	must := func(r rules.Rule) {
		if err := s.PutRule(r); err != nil {
			t.Fatalf("PutRule: %v", err)
		}
	}
	must(rules.Rule{ID: "zzz", Match: "a", SetCategoryID: "c", Order: 0})
	must(rules.Rule{ID: "aaa", Match: "b", SetCategoryID: "c", Order: 2})
	must(rules.Rule{ID: "mmm", Match: "d", SetCategoryID: "c", Order: 1})

	got, err := s.ListRules()
	if err != nil {
		t.Fatalf("ListRules: %v", err)
	}
	want := []string{"zzz", "mmm", "aaa"} // Order 0,1,2
	if len(got) != 3 {
		t.Fatalf("got %d rules, want 3", len(got))
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("precedence order = %v…, want %v at %d", got[i].ID, id, i)
		}
	}
}
