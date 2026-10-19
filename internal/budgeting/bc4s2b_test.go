// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestSweepDoesNotTakeRolloverOrPooledRolloverFunds(t *testing.T) {
	start := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	plain := domain.Budget{ID: "plain", Name: "Plain", Period: domain.PeriodMonthly, Limit: money.New(10000, "USD")}
	rollover := domain.Budget{ID: "rollover", Name: "Rollover", Period: domain.PeriodMonthly, Limit: money.New(10000, "USD"), Rollover: true, RolloverStart: start}
	in := ViewInput{
		Rates: currency.Rates{Base: "USD"}, Base: "USD",
		Period: ViewPeriod{Anchor: start, Now: start.AddDate(0, 0, 20), WeekStart: time.Monday},
	}
	cfg := SweepConfig{Enabled: true, BudgetIDs: []string{"plain", "rollover"}}

	plan, err := EvaluateSweep([]domain.Budget{plain, rollover}, in, cfg, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Total.Amount != 10000 || len(plan.Lines) != 1 || plan.Lines[0].BudgetID != "plain" {
		t.Fatalf("sweep with per-budget rollover = %+v; want only plain's 10000 cents", plan)
	}

	in.PoolRollover = true
	plan, err = EvaluateSweep([]domain.Budget{plain}, in, cfg, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Total.Amount != 0 || len(plan.Lines) != 0 {
		t.Fatalf("sweep with pooled rollover = %+v; want no funds swept", plan)
	}
}

func TestComputeSweepExcludesRolloverStatus(t *testing.T) {
	rollover := Status{
		Budget:    domain.Budget{ID: "rollover", Name: "Rollover", Period: domain.PeriodMonthly, Rollover: true},
		Remaining: money.New(15000, "USD"),
	}
	plan := ComputeSweep([]Status{rollover}, SweepConfig{Enabled: true, BudgetIDs: []string{"rollover"}}, "USD", nil)
	if plan.Total.Amount != 0 || len(plan.Lines) != 0 {
		t.Fatalf("rollover remainder swept = %+v; want none", plan)
	}
}
