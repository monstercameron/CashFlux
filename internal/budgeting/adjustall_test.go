// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func adjBudget(id string, limit int64, cur string) domain.Budget {
	return domain.Budget{ID: id, Name: id, Period: domain.PeriodMonthly, Limit: money.New(limit, cur)}
}

func TestAdjustAllPreviewRaise(t *testing.T) {
	budgets := []domain.Budget{adjBudget("a", 20000, "USD"), adjBudget("b", 5000, "USD")}

	p := AdjustAllPreview(budgets, 10)
	if p.Count() != 2 {
		t.Fatalf("Count = %d, want 2", p.Count())
	}
	if p.TotalBefore != 25000 || p.TotalAfter != 27500 {
		t.Errorf("totals = %d → %d, want 25000 → 27500", p.TotalBefore, p.TotalAfter)
	}
	if p.TotalDelta() != 2500 {
		t.Errorf("TotalDelta = %d, want 2500", p.TotalDelta())
	}
	if p.Lines[0].After != 22000 || p.Lines[1].After != 5500 {
		t.Errorf("lines = %d, %d, want 22000, 5500", p.Lines[0].After, p.Lines[1].After)
	}
	if p.Currency != "USD" || p.MixedCurrency {
		t.Errorf("currency = %q mixed=%v, want USD and not mixed", p.Currency, p.MixedCurrency)
	}
}

// The preview must be exactly what the write does — same rounding, same floor.
func TestAdjustedLimitRoundsHalfAwayAndFloorsAtOne(t *testing.T) {
	cases := []struct {
		limit int64
		pct   float64
		want  int64
	}{
		{100, 5, 105},
		{101, 5, 106},  // 5.05 rounds to 5
		{110, 5, 116},  // 5.5 rounds away from zero to 6
		{100, -50, 50}, //
		{100, -90, 10},
		{1, -90, 1}, // a lower may shrink a budget, never delete it
		{3, -90, 1},
	}
	for _, c := range cases {
		if got := AdjustedLimit(c.limit, c.pct); got != c.want {
			t.Errorf("AdjustedLimit(%d, %v) = %d, want %d", c.limit, c.pct, got, c.want)
		}
	}
}

// A budget with nothing to scale is not "affected", so it must not appear in the
// count the confirmation quotes.
func TestAdjustAllPreviewSkipsNonPositiveLimits(t *testing.T) {
	budgets := []domain.Budget{adjBudget("a", 20000, "USD"), adjBudget("zero", 0, "USD")}
	p := AdjustAllPreview(budgets, 10)
	if p.Count() != 1 || p.Lines[0].Budget.ID != "a" {
		t.Errorf("preview = %+v, want only budget a", p.Lines)
	}
}

// Budgets in different currencies cannot be totalled, and the form must know not
// to print a total that would be adding dollars to euros.
func TestAdjustAllPreviewFlagsMixedCurrency(t *testing.T) {
	budgets := []domain.Budget{adjBudget("a", 20000, "USD"), adjBudget("b", 5000, "EUR")}
	p := AdjustAllPreview(budgets, 10)
	if !p.MixedCurrency {
		t.Error("MixedCurrency = false for USD + EUR budgets")
	}
	if p.Currency != "" {
		t.Errorf("Currency = %q, want empty when the budgets disagree", p.Currency)
	}
	if p.Count() != 2 {
		t.Errorf("Count = %d, want 2 — the per-budget lines are still valid", p.Count())
	}
}

func TestValidAdjustPct(t *testing.T) {
	valid := []float64{5, -10, 0.5, AdjustMinPct, AdjustMaxPct}
	for _, v := range valid {
		if !ValidAdjustPct(v) {
			t.Errorf("ValidAdjustPct(%v) = false, want true", v)
		}
	}
	invalid := []float64{0, AdjustMinPct - 0.1, AdjustMaxPct + 0.1, -1000}
	for _, v := range invalid {
		if ValidAdjustPct(v) {
			t.Errorf("ValidAdjustPct(%v) = true, want false", v)
		}
	}
}

// Every reduction is asked about, because it takes money out of every plan at
// once; so is any large raise, which is far likelier to be a typo than a plan.
func TestIsLargeAdjust(t *testing.T) {
	for _, v := range []float64{-1, -50, 26, 500} {
		if !IsLargeAdjust(v) {
			t.Errorf("IsLargeAdjust(%v) = false, want true", v)
		}
	}
	for _, v := range []float64{1, 5, 25} {
		if IsLargeAdjust(v) {
			t.Errorf("IsLargeAdjust(%v) = true, want false", v)
		}
	}
}
