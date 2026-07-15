// SPDX-License-Identifier: MIT

package store

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// TestCategoryClassRoundTrip proves the additive CategoryClass field (BG2)
// survives a store put/get cycle, and that an unset field reads back as the
// safe flex default via Category.ClassOf.
func TestCategoryClassRoundTrip(t *testing.T) {
	s := newStore(t)
	if err := s.PutCategory(domain.Category{ID: "rent", Name: "Rent", Kind: domain.KindExpense, CategoryClass: domain.ClassFixed}); err != nil {
		t.Fatalf("put fixed: %v", err)
	}
	if err := s.PutCategory(domain.Category{ID: "dining", Name: "Dining", Kind: domain.KindExpense}); err != nil {
		t.Fatalf("put default: %v", err)
	}

	got, ok, err := s.GetCategory("rent")
	if err != nil || !ok {
		t.Fatalf("get rent: ok=%v err=%v", ok, err)
	}
	if got.CategoryClass != domain.ClassFixed {
		t.Errorf("rent class = %q, want fixed", got.CategoryClass)
	}
	if got.ClassOf() != domain.ClassFixed {
		t.Errorf("rent ClassOf = %q, want fixed", got.ClassOf())
	}

	dining, _, _ := s.GetCategory("dining")
	if dining.CategoryClass != "" {
		t.Errorf("unset class = %q, want empty", dining.CategoryClass)
	}
	if dining.ClassOf() != domain.ClassFlex {
		t.Errorf("unset ClassOf = %q, want flex default", dining.ClassOf())
	}
}
