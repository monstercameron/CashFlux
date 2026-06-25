// SPDX-License-Identifier: MIT

package runway

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// TestProjectLiquidDelegatesConsistently asserts that ProjectLiquid produces
// results identical to a direct Project call with the same arguments — verifying
// that the liquid-cash wrapper does not transform the inputs or outputs.
func TestProjectLiquidDelegatesConsistently(t *testing.T) {
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	rates := usd()
	recs := []domain.Recurring{
		rec("paycheck", 200000, "USD", domain.CadenceMonthly, time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)),
		rec("rent", -150000, "USD", domain.CadenceMonthly, time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)),
	}

	liquidStart := int64(50000)
	horizon := 30
	buffer := int64(0)

	want, err := Project(liquidStart, recs, from, horizon, buffer, rates)
	if err != nil {
		t.Fatalf("Project returned error: %v", err)
	}

	got, err := ProjectLiquid(liquidStart, recs, from, horizon, buffer, rates)
	if err != nil {
		t.Fatalf("ProjectLiquid returned error: %v", err)
	}

	// The two calls must agree on every field of Projection.
	if got.MinBalance != want.MinBalance {
		t.Errorf("MinBalance: ProjectLiquid=%d, Project=%d", got.MinBalance, want.MinBalance)
	}
	if got.MinDay != want.MinDay {
		t.Errorf("MinDay: ProjectLiquid=%d, Project=%d", got.MinDay, want.MinDay)
	}
	if got.BreachDay != want.BreachDay {
		t.Errorf("BreachDay: ProjectLiquid=%d, Project=%d", got.BreachDay, want.BreachDay)
	}
	if got.BreachShortfall != want.BreachShortfall {
		t.Errorf("BreachShortfall: ProjectLiquid=%d, Project=%d", got.BreachShortfall, want.BreachShortfall)
	}
	if len(got.Daily) != len(want.Daily) {
		t.Fatalf("Daily length: ProjectLiquid=%d, Project=%d", len(got.Daily), len(want.Daily))
	}
	for i := range got.Daily {
		if got.Daily[i] != want.Daily[i] {
			t.Errorf("Daily[%d]: ProjectLiquid=%+v, Project=%+v", i, got.Daily[i], want.Daily[i])
		}
	}
}

// TestProjectLiquidNoBreach verifies the no-breach case delegates correctly.
func TestProjectLiquidNoBreach(t *testing.T) {
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	rates := usd()
	recs := []domain.Recurring{
		rec("paycheck", 500000, "USD", domain.CadenceMonthly, time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)),
	}

	got, err := ProjectLiquid(100000, recs, from, 30, 0, rates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.WillBreach() {
		t.Errorf("expected no breach, got BreachDay=%d", got.BreachDay)
	}
}
