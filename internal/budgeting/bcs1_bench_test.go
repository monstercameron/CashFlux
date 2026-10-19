// SPDX-License-Identifier: MIT

package budgeting

import (
	"fmt"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
)

func bcs1Fixture() ([]domain.Budget, []domain.Category, []domain.Transaction, []domain.TxnLink) {
	budgets := make([]domain.Budget, 25)
	cats := make([]domain.Category, 25)
	for i := range budgets {
		id := fmt.Sprint(i)
		cats[i] = domain.Category{ID: id, Name: "Category " + id, Kind: domain.KindExpense}
		budgets[i] = domain.Budget{ID: id, CategoryID: id, Period: domain.PeriodMonthly, Limit: usd(100000)}
	}
	txns := make([]domain.Transaction, 50000)
	for i := range txns {
		txns[i] = domain.Transaction{ID: fmt.Sprint(i), CategoryID: fmt.Sprint(i % 25), Date: time.Date(2026, time.Month(i%12+1), 10, 0, 0, 0, 0, time.UTC), Amount: usd(-100)}
	}
	txns[1].Amount = usd(60)
	links := []domain.TxnLink{{ID: "refund", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"0", "1"}, Amount: usd(60)}}
	return budgets, cats, txns, links
}

func BenchmarkBCS1(b *testing.B) {
	budgets, cats, txns, links := bcs1Fixture()
	rates := currency.Rates{Base: "USD"}
	now := mustDate("2026-10-19")
	run := func(b *testing.B, kind string) {
		for i := 0; i < b.N; i++ {
			ledger := NewNettedLedger(txns, links)
			all := ledger.Transactions()
			switch kind {
			case "AnnualGrid":
				if _, err := BuildAnnualGrid(budgets, all, 2026, rates, time.Monday, now, nil); err != nil {
					b.Fatal(err)
				}
			case "Summary":
				for _, budget := range budgets {
					if _, err := EvaluateRollup(budget, all, mustDate("2026-10-01"), mustDate("2026-11-01"), rates, DefaultNearThreshold, nil); err != nil {
						b.Fatal(err)
					}
				}
			case "Hints":
				for _, budget := range budgets {
					if _, err := RecommendBudgetLimitWithContext(budget, nil, all, now, 3, usd(100000), rates, time.Monday, time.Time{}); err != nil {
						b.Fatal(err)
					}
				}
			case "SuggestBudgets":
				if _, err := SuggestBudgets(cats, all, now, 3, rates, MethodRecent); err != nil {
					b.Fatal(err)
				}
			case "Generate5030":
				if _, err := Generate5030WithRates(1000000, cats, all, now, rates); err != nil {
					b.Fatal(err)
				}
			}
		}
	}
	for _, kind := range []string{"AnnualGrid", "Summary", "Hints", "SuggestBudgets", "Generate5030"} {
		b.Run(kind, func(b *testing.B) { b.ReportAllocs(); run(b, kind) })
	}
}
