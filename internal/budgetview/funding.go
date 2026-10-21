// SPDX-License-Identifier: MIT

package budgetview

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
)

// FundingInput contains the already selected income lines and monthly plan facts
// needed to prepare the budget summary's funding read.
type FundingInput struct {
	IncomeLines []reports.CategorySpend
	IncomeError error
	Method      budgeting.Methodology
	Expected    int64
	RolledOver  int64
	Budgeted    int64
	Fixed       int64
	PlanKnown   bool
}

// FundingPreparation preserves the availability of income and assignments
// independently. A zero Received amount is meaningful only when IncomeKnown is
// true; Funding is usable only when both facts are known and the selected method
// has a funding read.
type FundingPreparation struct {
	IncomeKnown         bool
	Received            int64
	IncomeError         error
	AssignmentsKnown    bool
	Assigned            int64
	AssignmentError     error
	FundingKnown        bool
	FundingEligible     bool
	Funding             budgeting.FundingRead
	OverAssignedKnown   bool
	OverAssigned        int64
	OverAssignmentError error
}

// PrepareFunding aggregates category income and assignments with checked
// arithmetic. It never turns a conversion or overflow error into a zero fact.
func PrepareFunding(in FundingInput) FundingPreparation {
	out := FundingPreparation{IncomeKnown: in.IncomeError == nil, IncomeError: in.IncomeError}
	if out.IncomeKnown {
		for _, line := range in.IncomeLines {
			total, err := money.Add(out.Received, line.Amount)
			if err != nil {
				out.IncomeKnown = false
				out.IncomeError = fmt.Errorf("sum income categories: %w", err)
				out.Received = 0
				break
			}
			out.Received = total
		}
	}

	if in.PlanKnown {
		assigned, err := money.Add(in.Budgeted, in.Fixed)
		if err != nil {
			out.AssignmentError = fmt.Errorf("sum budget and fixed assignments: %w", err)
		} else {
			out.AssignmentsKnown = true
			out.Assigned = assigned
		}
	}
	out.FundingEligible = in.PlanKnown && in.Expected > 0 &&
		(in.Method == budgeting.MethodZeroBased || in.Method == budgeting.MethodSimple)
	if out.IncomeKnown && out.AssignmentsKnown && out.FundingEligible {
		out.FundingKnown = true
		out.Funding = budgeting.FundingRead{
			Expected: in.Expected,
			Received: out.Received,
			Assigned: out.Assigned,
			Fixed:    in.Fixed,
		}
	}
	if out.AssignmentsKnown && in.PlanKnown {
		pool, err := money.Add(in.Expected, in.RolledOver)
		if err == nil {
			if pool > 0 {
				toAssign, subErr := money.Sub(pool, out.Assigned)
				err = subErr
				if err == nil && toAssign < 0 {
					out.OverAssigned, err = money.Neg(toAssign)
				}
			}
			if err == nil {
				out.OverAssignedKnown = true
			}
		}
		if err != nil {
			out.OverAssignmentError = fmt.Errorf("calculate over-assignment: %w", err)
		}
	}
	return out
}
