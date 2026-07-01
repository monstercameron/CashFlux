// SPDX-License-Identifier: MIT

package engineenv

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestBudgetVars(t *testing.T) {
	defs := []customfields.Def{
		{Key: "priority", EntityType: "budget", Type: customfields.TypeNumber},
		{Key: "note", EntityType: "budget", Type: customfields.TypeText},       // non-numeric → ignored
		{Key: "tip", EntityType: "transaction", Type: customfields.TypeNumber}, // wrong entity → ignored
	}
	b := domain.Budget{ID: "b1", Custom: map[string]any{"priority": float64(3), "note": "x"}}

	// Over budget: spent 120, limit 100 → remaining -20, overspend 20, percent 120.
	got := BudgetVars(b, 120, 100, defs)
	want := map[string]float64{
		"spent": 120, "limit": 100, "remaining": -20, "overspend": 20, "percent": 120,
		"cf_budget_priority": 3,
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("BudgetVars[%q] = %v, want %v", k, got[k], v)
		}
	}
	if _, ok := got["cf_budget_note"]; ok {
		t.Error("non-numeric custom field should not surface")
	}
	if _, ok := got["cf_txn_tip"]; ok {
		t.Error("wrong-entity custom field should not surface")
	}

	// Under budget: no overspend, remaining positive.
	under := BudgetVars(domain.Budget{}, 40, 100, nil)
	if under["overspend"] != 0 {
		t.Errorf("under-budget overspend = %v, want 0", under["overspend"])
	}
	if under["remaining"] != 60 {
		t.Errorf("under-budget remaining = %v, want 60", under["remaining"])
	}

	// Zero limit must not divide by zero.
	zero := BudgetVars(domain.Budget{}, 10, 0, nil)
	if zero["percent"] != 0 {
		t.Errorf("zero-limit percent = %v, want 0", zero["percent"])
	}
}

func TestMerge(t *testing.T) {
	base := map[string]float64{"a": 1, "b": 2}
	over := map[string]float64{"b": 20, "c": 30}
	got := Merge(base, over)
	if got["a"] != 1 || got["b"] != 20 || got["c"] != 30 {
		t.Errorf("Merge = %v, want a=1 b=20 c=30", got)
	}
	// Inputs must be untouched.
	if base["b"] != 2 || len(over) != 2 {
		t.Error("Merge mutated an input map")
	}
}
