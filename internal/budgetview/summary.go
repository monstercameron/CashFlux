// SPDX-License-Identifier: MIT

package budgetview

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// SummarySpendingInput supplies the immutable selected-window spending inputs.
// Ledger must already be refund-netted; budget cadences do not change this range.
type SummarySpendingInput struct {
	Ledger          budgeting.NettedLedger
	Categories      []domain.Category
	Statuses        []budgeting.Status
	Start, End, Now time.Time
	Rates           currency.Rates
	Base            string
}

// SummarySpending preserves partial amounts independently from completeness.
// Known is false for an invalid range/reference time or aggregate overflow;
// Complete is false whenever a relevant scope/contribution could not be counted.
type SummarySpending struct {
	Amount          money.Money
	Known, Complete bool
	Issues          []budgeting.ViewIssue
}

// PrepareSummarySpending counts the union of evaluable spending-budget scopes
// over [Start,End), converting each selected transaction contribution once.
func PrepareSummarySpending(in SummarySpendingInput) SummarySpending {
	out := SummarySpending{Amount: money.Zero(in.Base), Known: true, Complete: true}
	if !in.End.After(in.Start) || in.Now.IsZero() {
		out.Known, out.Complete = false, false
		out.Issues = []budgeting.ViewIssue{{Operation: "resolve selected-period budget spending", Err: fmt.Errorf("spending requires an ordered window and reference time")}}
		return out
	}
	if in.Now.Before(in.Start) {
		return out
	}
	budgets := make([]domain.Budget, 0, len(in.Statuses))
	for _, status := range in.Statuses {
		if status.Error != nil {
			out.Complete = false
			out.Issues = append(out.Issues, status.Issues...)
			if len(status.Issues) == 0 {
				out.Issues = append(out.Issues, budgeting.ViewIssue{Operation: "evaluate budget", BudgetID: status.Budget.ID, Err: status.Error})
			}
		} else if !status.Budget.IsSaving() {
			budgets = append(budgets, status.Budget)
		}
	}
	union := budgeting.PrepareSpendUnion(in.Categories, budgets)
	for _, txn := range in.Ledger {
		if txn.Date.Before(in.Start) || !txn.Date.Before(in.End) {
			continue
		}
		contribution, matched, issues := union.Contribution(txn)
		out.Issues = append(out.Issues, issues...)
		if len(issues) > 0 {
			out.Complete = false
		}
		if !matched {
			continue
		}
		amount, err := in.Rates.Convert(contribution, in.Base)
		if err != nil {
			out.Complete = false
			out.Issues = append(out.Issues, budgeting.ViewIssue{Operation: "convert selected-period budget spending", Err: err})
			continue
		}
		total, err := money.Add(out.Amount.Amount, amount.Amount)
		if err != nil {
			out.Amount.Amount = 0
			out.Known, out.Complete = false, false
			out.Issues = append(out.Issues, budgeting.ViewIssue{Operation: "sum selected-period budget spending", Err: err})
			break
		}
		out.Amount.Amount = total
	}
	out.Issues = budgeting.DeduplicateViewIssues(out.Issues)
	return out
}

// HeadlinePercent returns the bounded spending meter percentage. Clamping before
// MulDiv keeps even MaxInt64-scale spending/plan inputs representable.
func HeadlinePercent(spent, plan int64) int {
	if spent <= 0 || plan <= 0 {
		return 0
	}
	if spent >= plan {
		return 100
	}
	pct, _ := money.MulDiv(spent, 100, plan, money.RoundTowardZero) // 0 < result < 100
	return int(pct)
}
