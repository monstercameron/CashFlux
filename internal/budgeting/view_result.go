// SPDX-License-Identifier: MIT

package budgeting

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/money"
)

// BudgetEvaluationError identifies a budget that cannot contribute reliable figures.
// Unwrap preserves missing-rate and overflow errors for callers.
type BudgetEvaluationError struct {
	BudgetID  string
	Operation string
	Err       error
}

// Error describes the failed budget calculation.
func (e *BudgetEvaluationError) Error() string {
	return fmt.Sprintf("budget %q %s: %v", e.BudgetID, e.Operation, e.Err)
}

// Unwrap returns the underlying calculation error.
func (e *BudgetEvaluationError) Unwrap() error { return e.Err }

// PeriodStatus describes the calendar state independently of financial health.
type PeriodStatus string

const (
	// PeriodPlanned marks a period that has not started.
	PeriodPlanned PeriodStatus = "planned"
	// PeriodEarly marks a live period too young for a useful pace signal.
	PeriodEarly PeriodStatus = "early"
	// PeriodActive marks a live period with enough elapsed time to assess pace.
	PeriodActive PeriodStatus = "active"
	// PeriodEnded marks a completed period.
	PeriodEnded PeriodStatus = "ended"
)

// RolloverView retains at most three prior periods plus current and lifetime totals.
type RolloverView struct {
	Prior                 []RolloverPeriodState
	Current               RolloverPeriodState
	Start                 time.Time
	StartAssumed          bool
	Periods               int
	TotalBase, TotalSpent money.Money
}

// DeduplicateViewIssues preserves the first diagnostic per budget, or per
// operation for diagnostics that do not belong to one budget.
func DeduplicateViewIssues(issues []ViewIssue) []ViewIssue {
	out := make([]ViewIssue, 0, len(issues))
	seen := make(map[string]bool)
	for _, issue := range issues {
		key := "budget:" + issue.BudgetID
		if issue.BudgetID == "" {
			key = "operation:" + issue.Operation
		}
		if !seen[key] {
			seen[key] = true
			out = append(out, issue)
		}
	}
	return out
}

func periodStatus(start, end, now time.Time, early bool) PeriodStatus {
	if now.Before(start) {
		return PeriodPlanned
	}
	if !now.Before(end) {
		return PeriodEnded
	}
	if early {
		return PeriodEarly
	}
	return PeriodActive
}
