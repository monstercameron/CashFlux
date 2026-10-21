// SPDX-License-Identifier: MIT

package budgeting

import (
	"fmt"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

var spendUnionBenchmarkResult int64

func spendUnionBenchmarkFixture(count, budgetCount int) ([]domain.Category, []domain.Budget, NettedLedger) {
	categories := make([]domain.Category, 200)
	for i := range categories {
		categories[i].ID = fmt.Sprintf("cat-%03d", i)
		if i >= 20 {
			categories[i].ParentID = categories[i%20].ID
		}
	}
	budgets := make([]domain.Budget, budgetCount)
	for i := range budgets {
		budgets[i] = domain.Budget{ID: fmt.Sprintf("budget-%03d", i), CategoryIDs: []string{categories[i%20].ID, categories[(i+1)%20].ID}}
		if i%10 == 0 {
			budgets[i].TrackedTags = []string{"trip"}
		}
	}
	rows := make(NettedLedger, count)
	for i := range rows {
		rows[i] = domain.Transaction{ID: fmt.Sprintf("txn-%05d", i), CategoryID: categories[i%200].ID, Amount: money.New(-100, "USD")}
		if i%3 == 0 {
			rows[i].Splits = []domain.CategorySplit{{CategoryID: categories[i%200].ID, Amount: money.New(-60, "USD")}, {CategoryID: categories[(i+1)%200].ID, Amount: money.New(-40, "USD")}}
		}
		if i%7 == 0 {
			rows[i].Tags = []string{"trip"}
		}
	}
	return categories, budgets, rows
}

// BenchmarkSpendUnion separates cold scope preparation from a prepared fold.
// Both variants include contribution matching; these are native model baselines,
// not an application navigation or cache-hit performance claim.
func BenchmarkSpendUnion(b *testing.B) {
	for _, size := range [][2]int{{3300, 50}, {30000, 250}} {
		categories, budgets, rows := spendUnionBenchmarkFixture(size[0], size[1])
		fold := func(union SpendUnion) int64 {
			var total int64
			for _, row := range rows {
				amount, matched, issues := union.Contribution(row)
				if !matched || len(issues) > 0 {
					b.Fatalf("fixture incomplete: matched=%v issues=%v", matched, issues)
				}
				total += amount.Amount // fixture bound is at most 3,000,000
			}
			return total
		}
		prepared := PrepareSpendUnion(categories, budgets)
		if got := fold(prepared); got != int64(size[0])*100 {
			b.Fatalf("fixture union=%d", got)
		}
		name := fmt.Sprintf("%dTxns_%dBudgets", size[0], size[1])
		b.Run(name+"/ColdPrepareAndFold", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				spendUnionBenchmarkResult = fold(PrepareSpendUnion(categories, budgets))
			}
		})
		b.Run(name+"/PreparedFold", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				spendUnionBenchmarkResult = fold(prepared)
			}
		})
	}
}
