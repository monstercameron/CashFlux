// SPDX-License-Identifier: MIT

package networthview

import (
	"math"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestAccountWindowsMatchesLegacyWindowFacts(t *testing.T) {
	since, until := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	accounts := []domain.Account{
		{ID: "asset", Currency: "USD", OpeningBalance: money.New(1000, "USD")},
		{ID: "debt-negative", Currency: "USD", Class: domain.ClassLiability, OpeningBalance: money.New(-900, "USD")},
		{ID: "debt-positive", Currency: "USD", Class: domain.ClassLiability, OpeningBalance: money.New(900, "USD")},
		{ID: "foreign", Currency: "EUR", OpeningBalance: money.New(200, "EUR")},
	}
	txns := []domain.Transaction{
		{AccountID: "asset", Date: since.Add(-time.Hour), Amount: money.New(50, "USD")},
		{AccountID: "asset", Date: since, Amount: money.New(-200, "USD")},
		{AccountID: "asset", Date: until.Add(-24 * time.Hour), Amount: money.New(25, "USD")},
		{AccountID: "asset", Date: until, Amount: money.New(700, "USD")},
		{AccountID: "debt-negative", Date: since.Add(time.Hour), Amount: money.New(200, "USD")},
		{AccountID: "debt-positive", Date: since.Add(time.Hour), Amount: money.New(-200, "USD")},
		{AccountID: "foreign", Date: since.Add(time.Hour), Amount: money.New(100, "EUR")},
	}
	accountsBefore, txnsBefore := append([]domain.Account(nil), accounts...), append([]domain.Transaction(nil), txns...)
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.5}}
	isAdjustment := func(tx domain.Transaction) bool { return tx.AccountID == "asset" && tx.Amount.Amount == 25 }
	got := AccountWindows(accounts, txns, rates, since, until, isAdjustment)
	if !reflect.DeepEqual(accounts, accountsBefore) || !reflect.DeepEqual(txns, txnsBefore) {
		t.Fatal("model mutated input")
	}
	if len(got) != len(accounts) {
		t.Fatalf("got %d rows", len(got))
	}
	want := []struct{ balance, base, move, start, end, flow, adj int64 }{
		{1575, 1575, -175, 1050, 875, -200, 25},
		{-700, -700, 200, -900, -700, 200, 0},
		{700, 700, 200, 900, 700, 200, 0},
		{300, 450, 150, 300, 450, 150, 0},
	}
	for i, row := range got {
		if !row.BalanceKnown() || !row.BaseBalanceKnown() || !row.WindowKnown() {
			t.Fatalf("row %s unavailable: %+v", row.AccountID, row)
		}
		facts := []int64{row.Balance.Amount, row.BaseBalance.Amount, row.Move.Amount, row.Start.Amount, row.End.Amount, row.Flow.Amount, row.Adjustment.Amount}
		w := want[i]
		if !reflect.DeepEqual(facts, []int64{w.balance, w.base, w.move, w.start, w.end, w.flow, w.adj}) {
			t.Errorf("row %s facts %v", row.AccountID, facts)
		}
	}
}

func TestAccountWindowsAvailabilityIsIndependent(t *testing.T) {
	since, until := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	accounts := []domain.Account{
		{ID: "missing-fx", Currency: "EUR", OpeningBalance: money.New(100, "EUR")},
		{ID: "bad-opening", Currency: "USD", OpeningBalance: money.New(100, "EUR")},
		{ID: "overflow", Currency: "USD", OpeningBalance: money.New(math.MaxInt64, "USD")},
	}
	txns := []domain.Transaction{
		{AccountID: "missing-fx", Date: since.Add(time.Hour), Amount: money.New(1, "EUR")},
		{AccountID: "overflow", Date: since.Add(time.Hour), Amount: money.New(1, "USD")},
	}
	got := AccountWindows(accounts, txns, currency.Rates{Base: "USD"}, since, until, nil)
	if !got[0].BalanceKnown() || got[0].BaseBalanceKnown() || got[0].WindowKnown() {
		t.Fatalf("FX facts conflated: %+v", got[0])
	}
	if got[0].Balance.Amount != 101 || got[0].Balance.Currency != "EUR" {
		t.Fatalf("native balance was lost: %+v", got[0])
	}
	if got[1].BalanceKnown() || got[1].WindowKnown() {
		t.Fatalf("bad opening became available: %+v", got[1])
	}
	if got[2].BalanceKnown() || got[2].WindowKnown() {
		t.Fatalf("overflow became available: %+v", got[2])
	}
	if _, err := money.Add(math.MaxInt64, 1); err == nil {
		t.Fatal("test precondition: expected overflow")
	}
}

func benchmarkAccountWindowsFixture(accountsN, transactionsN int) ([]domain.Account, []domain.Transaction) {
	accounts := make([]domain.Account, accountsN)
	txns := make([]domain.Transaction, transactionsN)
	for i := range accounts {
		accounts[i] = domain.Account{ID: stringID(i), Currency: "USD", OpeningBalance: money.New(10_000, "USD")}
	}
	for i := range txns {
		account := i % accountsN
		txns[i] = domain.Transaction{AccountID: stringID(account), Date: time.Date(2026, 1, 1+i%28, 0, 0, 0, 0, time.UTC), Amount: money.New(-1, "USD")}
	}
	return accounts, txns
}

func stringID(i int) string { return "account-" + strconv.Itoa(i) }

func BenchmarkAccountWindowPreparation(b *testing.B) {
	accounts, txns := benchmarkAccountWindowsFixture(100, 100_000)
	since, until := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	rates := currency.Rates{Base: "USD"}
	b.Run("legacy-per-account-scan", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			legacyWindowScan(accounts, txns, rates, since, until)
		}
	})
	b.Run("indexed-one-pass", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			AccountWindows(accounts, txns, rates, since, until, nil)
		}
	})
}

// legacyWindowScan mirrors the pre-extraction per-account reduction for an
// identical same-process comparison. It is benchmark-only and never used by
// production callers.
func legacyWindowScan(accounts []domain.Account, txns []domain.Transaction, rates currency.Rates, since, until time.Time) {
	for _, account := range accounts {
		var start, end int64
		for _, tx := range txns {
			if tx.AccountID != account.ID {
				continue
			}
			if tx.Date.Before(since) {
				start += tx.Amount.Amount
			}
			if tx.Date.Before(until) {
				end += tx.Amount.Amount
			}
		}
		if account.Class == domain.ClassLiability {
			_, _ = liabilityDelta(account, start, end)
		} else {
			_ = end - start
		}
		_, _ = rates.Convert(money.New(end, account.Currency), rates.Base)
	}
}
