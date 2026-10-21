// SPDX-License-Identifier: MIT

package budgetview

import (
	"errors"
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/reports"
)

func TestPrepareFundingPreservesKnownIncomeAndAssignments(t *testing.T) {
	got := PrepareFunding(FundingInput{
		IncomeLines: []reports.CategorySpend{{CategoryID: "pay", Amount: 696100}, {CategoryID: "other", Amount: 10}},
		Method:      budgeting.MethodZeroBased,
		Expected:    1070916,
		Budgeted:    1000000,
		Fixed:       70916,
		PlanKnown:   true,
	})
	if !got.IncomeKnown || !got.AssignmentsKnown || !got.FundingKnown {
		t.Fatalf("availability = income %v assignments %v funding %v", got.IncomeKnown, got.AssignmentsKnown, got.FundingKnown)
	}
	if got.Received != 696110 || got.Assigned != 1070916 {
		t.Fatalf("received/assigned = %d/%d, want 696110/1070916", got.Received, got.Assigned)
	}
	if got.Funding.Unfunded() != 374806 || got.Funding.Fixed != 70916 {
		t.Fatalf("funding read = %+v", got.Funding)
	}
}

func TestPrepareFundingDistinguishesKnownZeroFromIncomeFailure(t *testing.T) {
	zero := PrepareFunding(FundingInput{Method: budgeting.MethodSimple, Expected: 100, PlanKnown: true, Budgeted: 0})
	if !zero.IncomeKnown || zero.Received != 0 || !zero.FundingKnown {
		t.Fatalf("empty known income = %+v", zero)
	}

	fxErr := errors.New("missing FX rate")
	failed := PrepareFunding(FundingInput{Method: budgeting.MethodSimple, Expected: 100, IncomeError: fxErr, PlanKnown: true})
	if failed.IncomeKnown || failed.FundingKnown || failed.IncomeError == nil {
		t.Fatalf("failed income should remain unavailable: %+v", failed)
	}
	if failed.Received != 0 {
		t.Fatalf("unavailable display amount = %d, want no fabricated partial amount", failed.Received)
	}
}

func TestPrepareFundingRejectsCategoryAndAssignmentOverflow(t *testing.T) {
	income := PrepareFunding(FundingInput{
		IncomeLines: []reports.CategorySpend{{Amount: math.MaxInt64}, {Amount: 1}},
		PlanKnown:   true,
	})
	if income.IncomeKnown || income.FundingKnown || income.IncomeError == nil {
		t.Fatalf("overflowed category aggregation = %+v", income)
	}

	assigned := PrepareFunding(FundingInput{
		IncomeLines: []reports.CategorySpend{},
		Budgeted:    math.MaxInt64,
		Fixed:       1,
		PlanKnown:   true,
	})
	if !assigned.IncomeKnown || assigned.AssignmentsKnown || assigned.FundingKnown || assigned.AssignmentError == nil {
		t.Fatalf("overflowed assignment aggregation = %+v", assigned)
	}
}

func TestPrepareFundingLeavesPlanUnavailable(t *testing.T) {
	got := PrepareFunding(FundingInput{IncomeLines: []reports.CategorySpend{{Amount: 10}}, PlanKnown: false})
	if !got.IncomeKnown || got.AssignmentsKnown || got.FundingKnown {
		t.Fatalf("partial preparation = %+v", got)
	}
	if got.Funding != (budgeting.FundingRead{}) {
		t.Fatalf("unavailable funding read = %+v", got.Funding)
	}
}

func TestPrepareFundingPreservesSummaryMethodEligibility(t *testing.T) {
	got := PrepareFunding(FundingInput{
		IncomeLines: []reports.CategorySpend{{Amount: 50}}, Method: budgeting.MethodEnvelope,
		Expected: 100, PlanKnown: true,
	})
	if !got.IncomeKnown || !got.AssignmentsKnown || got.FundingEligible || got.FundingKnown {
		t.Fatalf("envelope funding should remain ineligible: %+v", got)
	}
}
