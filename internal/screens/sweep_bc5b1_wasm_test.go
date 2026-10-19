// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestSweepBC5B1BudgetEligibilityExplainsSkippedRows(t *testing.T) {
	tests := []struct {
		name     string
		budget   domain.Budget
		method   string
		eligible bool
	}{
		{name: "monthly spending", budget: domain.Budget{Period: domain.PeriodMonthly}, eligible: true},
		{name: "weekly", budget: domain.Budget{Period: domain.PeriodWeekly}},
		{name: "saving", budget: domain.Budget{Period: domain.PeriodMonthly, Direction: domain.DirectionSave}},
		{name: "own rollover", budget: domain.Budget{Period: domain.PeriodMonthly, Rollover: true}},
		{name: "inherited envelope", budget: domain.Budget{Period: domain.PeriodMonthly}, method: string(budgeting.MethodEnvelope)},
		{name: "envelope override", budget: domain.Budget{Period: domain.PeriodMonthly, Methodology: string(budgeting.MethodEnvelope)}, eligible: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := sweepBudgetEligible(test.budget, test.method) == ""
			if got != test.eligible {
				t.Fatalf("eligible = %v, want %v", got, test.eligible)
			}
		})
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(sweepBudgetCheckRow, sweepBudgetRowProps{ID: "weekly-b1", Name: "Weekly", Disabled: true, Reason: "not available"}))
	row := byTestID(f, "label", "sweep-config-budget-weekly-b1")
	if row == nil || len(row.Children()) == 0 || row.Children()[0].Attr("disabled") == "" {
		if row != nil && len(row.Children()) > 0 {
			t.Fatalf("ineligible sweep row must render a disabled checkbox: child tag=%s disabled=%q testid=%q", row.Children()[0].Tag(), row.Children()[0].Attr("disabled"), row.Children()[0].Attr("data-testid"))
		}
		t.Fatal("ineligible sweep row must render a disabled checkbox")
	}
}

func TestSweepBC5B1SweptBudgetMarkerIsSharedAcrossMemberLenses(t *testing.T) {
	const month = "2099-02"
	ids := []string{"bc5b1-shared-budget", "bc5b1-alice-budget"}
	markSweepBudgetIDs(month, []string{ids[0]}, "swept")
	got := sweepBudgetIDs(month, ids)
	if len(got) != 1 || got[0] != ids[0] {
		t.Fatalf("swept IDs = %v, want the shared budget only", got)
	}
	if uistate.KVGet(sweepBudgetStatusKey(month, ids[0])) != "swept" {
		t.Fatal("swept marker was not persisted through the shared KV store")
	}
}
