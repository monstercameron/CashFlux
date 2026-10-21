// SPDX-License-Identifier: MIT

package ledger

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

var benchmarkAccountValueSeries []money.Money

func BenchmarkAccountValuePreparation(b *testing.B) {
	for _, size := range []struct {
		name                    string
		accounts, rows, cutoffs int
	}{
		{"dashboard-history", 24, 24000, 8},
		{"long-history", 80, 60000, 12},
	} {
		b.Run(size.name, func(b *testing.B) {
			start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
			accounts := make([]domain.Account, size.accounts)
			for i := range accounts {
				code := "USD"
				if i%4 == 0 {
					code = "EUR"
				}
				a := domain.Account{ID: fmt.Sprint(i), Currency: code, OpeningBalance: money.New(100000, code)}
				if i%5 == 0 {
					a.Class = domain.ClassLiability
					if i%2 == 0 {
						a.OpeningBalance.Amount = -100000
					}
				}
				a.Archived, a.ExcludeFromNetWorth = i%17 == 0, i%13 == 0
				accounts[i] = a
			}
			all := make([]domain.Transaction, size.rows)
			for i := range all {
				a := accounts[i%len(accounts)]
				// Date order intentionally differs from source/addition order.
				all[i] = domain.Transaction{AccountID: a.ID, Date: start.AddDate(0, -(i * 31 % 120), i%27), Amount: money.New(int64(i%101)-50, a.Currency)}
			}
			cutoffs := make([]time.Time, size.cutoffs)
			for i := range cutoffs {
				cutoffs[i] = start.AddDate(0, -(i * 7 % size.cutoffs), 0)
			}
			rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.1}}
			policy := accountTotalsPolicy{}
			want, err := frozenAccountValueSeries(accounts, all, cutoffs, rates, policy, "net worth")
			if err != nil {
				b.Fatal(err)
			}
			got, err := accountValueSeries(accounts, all, cutoffs, rates, policy, "net worth")
			if err != nil || !reflect.DeepEqual(got, want) {
				b.Fatalf("fixture parity: got=%v err=%v want=%v", got, err, want)
			}
			var eligible, matching, beforeCutoffs int64
			counts := make(map[string]int64)
			for _, txn := range all {
				counts[txn.AccountID]++
			}
			for _, a := range accounts {
				if a.CountsInNetWorth() {
					eligible++
					matching += counts[a.ID]
				}
			}
			for _, c := range cutoffs {
				for _, txn := range all {
					if txn.Date.Before(c) {
						beforeCutoffs++
					}
				}
			}
			oldVisits := int64(len(cutoffs)*len(all)) + eligible*beforeCutoffs
			newVisits := int64(len(all)) + int64(len(cutoffs))*matching
			b.Run("frozen-full-ledger", func(b *testing.B) {
				b.ReportAllocs()
				b.ReportMetric(float64(oldVisits), "candidate-rowvisits/op")
				for range b.N {
					benchmarkAccountValueSeries, err = frozenAccountValueSeries(accounts, all, cutoffs, rates, policy, "net worth")
					if err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("indexed-source-order", func(b *testing.B) {
				b.ReportAllocs()
				b.ReportMetric(float64(newVisits), "candidate-rowvisits/op")
				for range b.N {
					benchmarkAccountValueSeries, err = accountValueSeries(accounts, all, cutoffs, rates, policy, "net worth")
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}
