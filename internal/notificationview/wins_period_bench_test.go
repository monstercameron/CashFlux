// SPDX-License-Identifier: MIT

package notificationview

import (
	"fmt"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

var benchmarkWinsBudgetCount int

// BenchmarkWinsMonthlyBudgetProjection isolates the old B*N budget-date checks
// from the N+B*K completed-month projection. The fixture retains ten years of
// history, 180 budgets, mixed owners, duplicate categories and saving transfers;
// net-worth and no-spend work is intentionally outside this measured phase.
func BenchmarkWinsMonthlyBudgetProjection(b *testing.B) {
	const months, rowsPerMonth, budgetCount = 120, 500, 180
	start := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	all := make(budgeting.NettedLedger, 0, months*rowsPerMonth)
	for month := range months {
		at := start.AddDate(0, -month, 0)
		for row := range rowsPerMonth {
			txn := domain.Transaction{ID: fmt.Sprintf("%d-%d", month, row), Date: at.AddDate(0, 0, row%27), CategoryID: fmt.Sprintf("cat-%d", row%60), MemberID: fmt.Sprintf("member-%d", row%3), Amount: money.New(-100-int64(row%50), "USD")}
			if row%7 == 0 {
				txn.TransferAccountID = "savings"
			}
			if row%11 == 0 {
				txn.Tags = []string{"trip"}
			}
			all = append(all, txn)
		}
	}
	budgets := make([]domain.Budget, budgetCount)
	for i := range budgets {
		budget := domain.Budget{ID: fmt.Sprint(i), Period: domain.PeriodMonthly, CategoryID: fmt.Sprintf("cat-%d", i%60), Limit: money.New(100000, "USD"), Scope: domain.ScopeShared}
		if i%3 == 0 {
			budget.Scope = domain.ScopeIndividual
			budget.OwnerID = fmt.Sprintf("member-%d", i%3)
		}
		if i%7 == 0 {
			budget.Direction = domain.DirectionSave
		}
		if i%11 == 0 {
			budget.TrackedTags = []string{"trip"}
		}
		budgets[i] = budget
	}
	rates := currency.Rates{Base: "USD"}
	want := frozenMonthlyBudgetWins(budgets, all, start, end, rates)
	if got := keptMonthlyBudgetWins(budgets, all, start, end, rates); got != want || want == 0 {
		b.Fatalf("fixture parity failed: got=%d want=%d", got, want)
	}
	monthlyRows := len(budgetWinMonth(all, start, end))
	b.Run("frozen-full-ledger", func(b *testing.B) {
		b.ReportAllocs()
		b.ReportMetric(float64(len(budgets)*len(all)), "candidate-datechecks/op")
		b.ReportMetric(float64(want), "kept-budgets/op")
		for range b.N {
			benchmarkWinsBudgetCount = frozenMonthlyBudgetWins(budgets, all, start, end, rates)
		}
	})
	b.Run("shared-completed-month", func(b *testing.B) {
		b.ReportAllocs()
		b.ReportMetric(float64(len(all)+len(budgets)*monthlyRows), "candidate-datechecks/op")
		b.ReportMetric(float64(want), "kept-budgets/op")
		for range b.N {
			benchmarkWinsBudgetCount = keptMonthlyBudgetWins(budgets, all, start, end, rates)
		}
	})
}
