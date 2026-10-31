// SPDX-License-Identifier: MIT

package txnfilter

import (
	"fmt"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

var compiledBenchmarkMatches int
var compiledBenchmarkPresets PresetCounts

func compiledBenchmarkRows(n int) []domain.Transaction {
	rows := make([]domain.Transaction, n)
	for i := range rows {
		rows[i] = domain.Transaction{ID: fmt.Sprint(i), AccountID: fmt.Sprintf("account-%d", i%4), CategoryID: fmt.Sprintf("category-%d", i%3),
			MemberID: fmt.Sprintf("member-%d", i%2), Source: "manual", Payee: "Merchant", Desc: "Merchant statement charge", Tags: []string{"Coffee", "needs-review"},
			Date: time.Date(2026, 10, i%28+1, 12, 0, 0, 0, time.UTC), Amount: money.New(-int64(1000+i%3), "USD")}
	}
	return rows
}

func BenchmarkCompiledCriteriaPreparation(b *testing.B) {
	for _, size := range []int{1000, 10000} {
		rows := compiledBenchmarkRows(size)
		for _, tc := range []struct {
			name string
			c    Criteria
		}{
			{"plain-search", Criteria{Text: "merchant"}},
			{"numeric-search", Criteria{Text: "$10.00"}},
			{"multi-facets", Criteria{Text: "merchant", Accounts: "account-0,account-1,account-2", Categories: "category-0,category-1", Members: "member-0,member-1", Sources: "manual,imported", Tags: "coffee,needs-review"}},
		} {
			b.Run(fmt.Sprintf("%d/%s/prepared", size, tc.name), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					filter := compileCriteria(tc.c, Labels{})
					count := 0
					for _, row := range rows {
						if filter.matches(row) {
							count++
						}
					}
					compiledBenchmarkMatches = count
				}
			})
			b.Run(fmt.Sprintf("%d/%s/previous-per-row-parsing", size, tc.name), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					filter := legacyCompile(tc.c, Labels{})
					count := 0
					for _, row := range rows {
						if filter.matches(row) {
							count++
						}
					}
					compiledBenchmarkMatches = count
				}
			})
		}
		b.Run(fmt.Sprintf("%d/preset-counts", size), func(b *testing.B) {
			c := Criteria{Text: "merchant", Accounts: "account-0,account-1,account-2", Members: "member-0,member-1"}
			s := PresetScope{ReviewTag: "needs-review", LargeMinor: 1000, MonthStart: d("2026-10-01"), MonthEnd: d("2026-11-01")}
			labels := Labels{BaseCurrency: "USD", ToBase: currency.Rates{Base: "USD"}.ToBase}
			b.Run("prepared", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					compiledBenchmarkPresets = CountPresetsWithLabels(rows, c, s, labels)
				}
			})
			b.Run("previous-per-row-parsing", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					compiledBenchmarkPresets = legacyPresetCounts(rows, c, s, labels)
				}
			})
		})
	}
}
