// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TestSmoothingLandingOffset: in the landing period the posted smoothed bill is
// offset (so the row reads on-pace); unposted bills and off periods offset
// nothing; non-smoothed recurrings never offset.
func TestSmoothingLandingOffset(t *testing.T) {
	juneStart := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	julyStart := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	augStart := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	budget := domain.Budget{ID: "b1", CategoryID: "insurance", Limit: money.New(20000, "USD"), Period: domain.PeriodMonthly}
	annual := domain.Recurring{
		ID: "r1", Label: "Car insurance", CategoryID: "insurance",
		Amount: money.New(-60000, "USD"), Cadence: domain.CadenceYearly,
		NextDue: time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC), SmoothIntoBudgets: true,
	}
	posted := []domain.Transaction{
		{ID: "t1", CategoryID: "insurance", Amount: money.New(-60000, "USD"),
			Date: time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)},
	}

	// Landing month, bill posted: full magnitude offsets.
	if got := SmoothingLandingOffset(budget, []domain.Recurring{annual}, posted, juneStart, julyStart); got != 60000 {
		t.Errorf("landing offset = %d, want 60000", got)
	}
	// Landing month, bill NOT posted yet: nothing offsets (never hide real spending).
	if got := SmoothingLandingOffset(budget, []domain.Recurring{annual}, nil, juneStart, julyStart); got != 0 {
		t.Errorf("unposted offset = %d, want 0", got)
	}
	// Off month: no landing, no offset.
	if got := SmoothingLandingOffset(budget, []domain.Recurring{annual}, posted, julyStart, augStart); got != 0 {
		t.Errorf("off-month offset = %d, want 0", got)
	}
	// Flag off: never offsets.
	plain := annual
	plain.SmoothIntoBudgets = false
	if got := SmoothingLandingOffset(budget, []domain.Recurring{plain}, posted, juneStart, julyStart); got != 0 {
		t.Errorf("non-smoothed offset = %d, want 0", got)
	}
}

// TestApplySmoothingOffset: figures shift, percent/state re-derive, and the
// offset never drives Spent below zero.
func TestApplySmoothingOffset(t *testing.T) {
	st := Status{
		Spent:     money.New(70000, "USD"), // $700 spent (annual bill landed)
		Remaining: money.New(-50000, "USD"),
		Percent:   350,
		State:     StateOver,
	}
	adj := ApplySmoothingOffset(st, 60000, DefaultNearThreshold)
	if adj.Spent.Amount != 10000 || adj.Remaining.Amount != 10000 {
		t.Errorf("adjusted = spent %d remaining %d, want 10000/10000", adj.Spent.Amount, adj.Remaining.Amount)
	}
	if adj.State != StateOK || adj.Percent != 50 {
		t.Errorf("adjusted state/percent = %s/%d, want ok/50", adj.State, adj.Percent)
	}
	// Offset larger than spent clamps to zero spent.
	huge := ApplySmoothingOffset(st, 999999, DefaultNearThreshold)
	if huge.Spent.Amount != 0 {
		t.Errorf("clamped spent = %d, want 0", huge.Spent.Amount)
	}
	// Zero offset is a no-op.
	same := ApplySmoothingOffset(st, 0, DefaultNearThreshold)
	if same.Spent != st.Spent || same.Remaining != st.Remaining || same.State != st.State {
		t.Errorf("zero offset changed the status")
	}
}
