// SPDX-License-Identifier: MIT

package coverformula

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestEvalInBudget(t *testing.T) {
	now := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	// Budget: limit $100, one expense of $130 in its category this month → over by $30.
	b := domain.Budget{
		ID: "b1", CategoryID: "cat", Period: domain.PeriodMonthly,
		Limit: money.New(10000, "USD"), Custom: map[string]any{"priority": float64(4)},
	}
	txns := []domain.Transaction{{ID: "t1", CategoryID: "cat", Date: now, Amount: money.New(-13000, "USD")}}
	defs := []customfields.Def{{Key: "priority", EntityType: "budget", Type: customfields.TypeNumber}}
	c := Context{
		Base:  map[string]float64{"income": 5000},
		Txns:  txns, Rates: currency.Rates{Base: "USD"}, Now: now, WeekStart: time.Sunday, Defs: defs,
	}

	cases := []struct {
		expr string
		want float64
	}{
		{"overspend", 30},         // spent 130 − limit 100
		{"remaining", -30},        // limit − spent
		{"spent", 130},            //
		{"limit", 100},            //
		{"cf_budget_priority", 4}, // this budget's own value
		{"cf_budget_priority * 10", 40},
		{"income", 5000},                    // global surface still reachable
		{"clamp(overspend, 0, 25)", 25},     // language functions work in context
		{"if(overspend > 0, overspend, 0)", 30},
		{"", 0}, // empty → 0
	}
	for _, tc := range cases {
		got, err := c.EvalInBudget(tc.expr, b)
		if err != nil {
			t.Errorf("EvalInBudget(%q) error: %v", tc.expr, err)
			continue
		}
		if got != tc.want {
			t.Errorf("EvalInBudget(%q) = %v, want %v", tc.expr, got, tc.want)
		}
	}

	// AmountMinor converts the major result back to cents and clamps at 0.
	if m, _ := c.AmountMinor("overspend", b); m != 3000 {
		t.Errorf("AmountMinor(overspend) = %d, want 3000", m)
	}
	if m, _ := c.AmountMinor("remaining", b); m != 0 { // -30 → clamped to 0
		t.Errorf("AmountMinor(remaining) = %d, want 0 (clamped)", m)
	}
	// Weight rounds to a non-negative int.
	if w, _ := c.Weight("cf_budget_priority", b); w != 4 {
		t.Errorf("Weight(cf_budget_priority) = %d, want 4", w)
	}

	// A malformed formula surfaces an error.
	if _, err := c.EvalInBudget("overspend +", b); err == nil {
		t.Error("expected a parse error for a malformed formula")
	}
}
