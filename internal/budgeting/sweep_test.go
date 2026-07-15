// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func sweepStatus(id, name string, remainingMinor int64) Status {
	return Status{
		Budget:    domain.Budget{ID: id, Name: name},
		Remaining: money.New(remainingMinor, "USD"),
	}
}

func TestComputeSweep(t *testing.T) {
	tests := []struct {
		name        string
		statuses    []Status
		cfg         SweepConfig
		allow       func(string) bool
		wantTotal   int64
		wantLines   int
		wantBlocked bool
	}{
		{
			name: "sums positive leftovers of participating budgets only",
			statuses: []Status{
				sweepStatus("groceries", "Groceries", 5000),
				sweepStatus("dining", "Dining", 2000),
				sweepStatus("gas", "Gas", 700),
			},
			cfg:       SweepConfig{Enabled: true, BudgetIDs: []string{"groceries", "dining", "gas"}, TargetGoalID: "emg"},
			wantTotal: 7700,
			wantLines: 3,
		},
		{
			name: "excludes non-participating budgets",
			statuses: []Status{
				sweepStatus("groceries", "Groceries", 5000),
				sweepStatus("dining", "Dining", 2000),
			},
			cfg:       SweepConfig{Enabled: true, BudgetIDs: []string{"groceries"}, TargetGoalID: "emg"},
			wantTotal: 5000,
			wantLines: 1,
		},
		{
			name: "overspent budget contributes nothing",
			statuses: []Status{
				sweepStatus("groceries", "Groceries", -1500),
				sweepStatus("dining", "Dining", 2000),
			},
			cfg:       SweepConfig{Enabled: true, BudgetIDs: []string{"groceries", "dining"}, TargetGoalID: "emg"},
			wantTotal: 2000,
			wantLines: 1,
		},
		{
			name: "exactly spent contributes nothing",
			statuses: []Status{
				sweepStatus("groceries", "Groceries", 0),
			},
			cfg:       SweepConfig{Enabled: true, BudgetIDs: []string{"groceries"}, TargetGoalID: "emg"},
			wantTotal: 0,
			wantLines: 0,
		},
		{
			name:      "disabled config sweeps nothing",
			statuses:  []Status{sweepStatus("groceries", "Groceries", 5000)},
			cfg:       SweepConfig{Enabled: false, BudgetIDs: []string{"groceries"}, TargetGoalID: "emg"},
			wantTotal: 0,
			wantLines: 0,
		},
		{
			name:        "integrity gate blocks the plan",
			statuses:    []Status{sweepStatus("groceries", "Groceries", 5000)},
			cfg:         SweepConfig{Enabled: true, BudgetIDs: []string{"groceries"}, TargetGoalID: "emg"},
			allow:       func(string) bool { return false },
			wantTotal:   5000,
			wantLines:   1,
			wantBlocked: true,
		},
		{
			name:        "gate allows when goal is healthy",
			statuses:    []Status{sweepStatus("groceries", "Groceries", 5000)},
			cfg:         SweepConfig{Enabled: true, BudgetIDs: []string{"groceries"}, TargetGoalID: "emg"},
			allow:       func(string) bool { return true },
			wantTotal:   5000,
			wantLines:   1,
			wantBlocked: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := ComputeSweep(tt.statuses, tt.cfg, "USD", tt.allow)
			if plan.Total.Amount != tt.wantTotal {
				t.Errorf("Total = %d, want %d", plan.Total.Amount, tt.wantTotal)
			}
			if len(plan.Lines) != tt.wantLines {
				t.Errorf("Lines = %d, want %d", len(plan.Lines), tt.wantLines)
			}
			if plan.Blocked != tt.wantBlocked {
				t.Errorf("Blocked = %v, want %v", plan.Blocked, tt.wantBlocked)
			}
			if tt.wantTotal > 0 && plan.Total.Currency != "USD" {
				t.Errorf("Total currency = %q, want USD", plan.Total.Currency)
			}
		})
	}
}

func TestSweepSuppressesRollover(t *testing.T) {
	// A budget with rollover enabled that ALSO participates in the sweep: sweep
	// wins (mutual exclusion, sweep precedence).
	cfg := SweepConfig{Enabled: true, BudgetIDs: []string{"groceries"}}
	if !cfg.SuppressesRollover("groceries") {
		t.Error("participating budget should have rollover suppressed by sweep")
	}
	if cfg.SuppressesRollover("dining") {
		t.Error("non-participating budget should keep its rollover")
	}
	// A disabled sweep never suppresses rollover.
	off := SweepConfig{Enabled: false, BudgetIDs: []string{"groceries"}}
	if off.SuppressesRollover("groceries") {
		t.Error("disabled sweep must not suppress rollover")
	}
}

func TestSweepPlanHelpers(t *testing.T) {
	empty := ComputeSweep(nil, SweepConfig{Enabled: true}, "USD", nil)
	if empty.HasLeftover() {
		t.Error("empty plan should not report leftover")
	}
	if empty.BudgetCount() != 0 {
		t.Errorf("BudgetCount = %d, want 0", empty.BudgetCount())
	}
	full := ComputeSweep(
		[]Status{sweepStatus("g", "Groceries", 5000)},
		SweepConfig{Enabled: true, BudgetIDs: []string{"g"}, TargetGoalID: "emg"},
		"USD", nil)
	if !full.HasLeftover() {
		t.Error("plan with leftover should report HasLeftover")
	}
	if full.BudgetCount() != 1 {
		t.Errorf("BudgetCount = %d, want 1", full.BudgetCount())
	}
}
