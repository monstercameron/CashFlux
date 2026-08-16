// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import "testing"

// The allocation read is the arithmetic behind "how much of my income have I
// budgeted?" — the question the /budgets hero never answered. These tests pin
// the state machine and the bar geometry, including the states the adversarial
// design pass found unhandled.

func allocTestView(income, rolled, budgeted, savings int64) budgetView {
	return budgetView{Base: "USD", BannerIncome: income, RolledOver: rolled, TotalLimit: budgeted, SavingsAssigned: savings}
}

func TestResolveAllocationStates(t *testing.T) {
	cases := []struct {
		name                            string
		income, rolled, budgeted, saved int64
		wantState                       string
		wantPct                         int
		wantUnassigned                  int64
	}{
		{"no income at all", 0, 0, 500000, 0, "", 0, 0},
		{"income but no budgets yet", 590000, 0, 0, 0, "empty", 0, 590000},
		{"comfortably under", 590000, 0, 300000, 0, "under", 50, 290000},
		{"exactly allocated", 590000, 0, 590000, 0, "exact", 100, 0},
		{"over income", 590000, 0, 958250, 0, "over", 162, -368250},
		{"savings counts toward the plan", 590000, 0, 300000, 290000, "exact", 100, 0},
		{"rollover raises the pool", 500000, 90000, 590000, 0, "exact", 100, 0},
		{"wildly over", 100000, 0, 900000, 0, "over", 900, -800000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := resolveAllocation(allocTestView(tc.income, tc.rolled, tc.budgeted, tc.saved))
			if r.State != tc.wantState {
				t.Errorf("State = %q, want %q", r.State, tc.wantState)
			}
			if r.PlanPct != tc.wantPct {
				t.Errorf("PlanPct = %d, want %d", r.PlanPct, tc.wantPct)
			}
			if r.Unassigned != tc.wantUnassigned {
				t.Errorf("Unassigned = %d, want %d", r.Unassigned, tc.wantUnassigned)
			}
		})
	}
}

// TestResolveAllocationNoIncomeRendersNothing is the guard that keeps the block
// off the page entirely for a household that has not set a basis: the component
// returns early on Pool <= 0, so nothing below it may be relied upon.
func TestResolveAllocationNoIncomeRendersNothing(t *testing.T) {
	r := resolveAllocation(allocTestView(0, 0, 500000, 0))
	if r.Pool > 0 {
		t.Fatalf("Pool = %d, want 0 so the block is skipped", r.Pool)
	}
	if r.State != "" {
		t.Errorf("State = %q, want empty — nothing is resolved without income", r.State)
	}
}

// TestAllocationBarSplitsAtTheIncomeTick is the fix for the first draft's worst
// flaw: the whole bar rendered in the healthy accent even when the plan ran far
// past income, so a bar that was 100% green sat under a caption saying "more
// than you earn". The reader believes the colour.
func TestAllocationBarSplitsAtTheIncomeTick(t *testing.T) {
	r := resolveAllocation(allocTestView(590000, 0, 958250, 0))
	if r.OverPct <= 0 {
		t.Fatal("an over-income plan must render a striped overflow segment")
	}
	if r.MarkerPct < 0 {
		t.Fatal("an over-income plan must mark where income runs out")
	}
	if r.BudgetedPct != r.MarkerPct {
		t.Errorf("the healthy fill must stop at the tick: fill=%d tick=%d", r.BudgetedPct, r.MarkerPct)
	}
	// 590000/958250 ≈ 61.6% — the tick sits where income runs out.
	if r.MarkerPct != 61 {
		t.Errorf("MarkerPct = %d, want 61", r.MarkerPct)
	}
	if got := r.BudgetedPct + r.OverPct; got != 100 {
		t.Errorf("the two halves of the plan must fill the track: %d", got)
	}
}

// TestAllocationBarHasNoTickWhenUnder: when the plan is under, the END of the
// track already IS the income pool, so a tick at 100% would mark a boundary the
// track edge draws for free. (The design critique argued for always showing it;
// rejected for this reason.)
func TestAllocationBarHasNoTickWhenUnder(t *testing.T) {
	for _, tc := range []struct {
		name             string
		income, budgeted int64
	}{
		{"way under", 590000, 10000},
		{"just under", 590000, 589000},
		{"exact", 590000, 590000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := resolveAllocation(allocTestView(tc.income, 0, tc.budgeted, 0))
			if r.MarkerPct >= 0 {
				t.Errorf("MarkerPct = %d, want -1 — the track edge already marks income", r.MarkerPct)
			}
			if r.OverPct != 0 {
				t.Errorf("OverPct = %d, want 0", r.OverPct)
			}
		})
	}
}

// TestAllocationSegmentsAlwaysFillTheTrack guards against a rounding sliver of
// bare track showing through, which reads as a fourth unlabelled category.
func TestAllocationSegmentsAlwaysFillTheTrack(t *testing.T) {
	cases := [][4]int64{
		{590000, 0, 300000, 0},
		{590000, 0, 300000, 150000},
		{590000, 0, 958250, 0},
		{333333, 0, 111111, 77777},
		{100000, 0, 900000, 0},
		{590000, 90000, 300000, 40000},
	}
	for _, c := range cases {
		r := resolveAllocation(allocTestView(c[0], c[1], c[2], c[3]))
		total := r.BudgetedPct + r.SavingsPct + r.OverPct + r.GapPct
		if total != 100 {
			t.Errorf("income=%d rolled=%d budgeted=%d saved=%d: segments total %d%%, want 100%%",
				c[0], c[1], c[2], c[3], total)
		}
		for _, seg := range []int{r.BudgetedPct, r.SavingsPct, r.OverPct, r.GapPct} {
			if seg < 0 {
				t.Errorf("income=%d budgeted=%d: negative segment %d", c[0], c[2], seg)
			}
		}
	}
}

// TestAllocationEmptyStateIsNotAFailureReport: an income basis with no budgets
// is day one, not a household that has fallen behind. It must be distinguishable
// from "under" so the copy can greet rather than scold.
func TestAllocationEmptyStateIsNotAFailureReport(t *testing.T) {
	empty := resolveAllocation(allocTestView(700000, 0, 0, 0))
	if empty.State != "empty" {
		t.Fatalf("State = %q, want empty", empty.State)
	}
	started := resolveAllocation(allocTestView(700000, 0, 1, 0))
	if started.State != "under" {
		t.Fatalf("one budget must leave the empty state, got %q", started.State)
	}
}
