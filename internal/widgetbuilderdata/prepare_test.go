// SPDX-License-Identifier: MIT

package widgetbuilderdata

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/cardgraph"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestPreparePreservesBuilderDatasetContract(t *testing.T) {
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	snapshot := Snapshot{
		At: now, BaseCurrency: "USD",
		Categories: []domain.Category{{ID: "food", Name: "Food"}},
		Transactions: []domain.Transaction{
			{ID: "expense", Date: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), Payee: "Market", CategoryID: "food", Amount: money.New(-1234, "USD")},
			{ID: "income", Date: time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC), Amount: money.New(5000, "USD")},
			{ID: "transfer", Date: now, Payee: "Move", TransferAccountID: "other", Amount: money.New(-900, "USD")},
		},
		Accounts: []domain.Account{
			{ID: "checking", Name: "Checking", Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(10000, "USD")},
			{ID: "archived", Name: "Old", Type: domain.TypeSavings, Currency: "USD", Archived: true},
		},
		Budgets: []domain.Budget{{Name: "Groceries", Limit: money.New(25000, "USD")}},
		Goals:   []domain.Goal{{Name: "Buffer", TargetAmount: money.New(50000, "USD"), CurrentAmount: money.New(12500, "USD")}},
		Tasks:   []domain.Task{{Title: "Review", Status: domain.StatusOpen}},
	}

	prepared, err := Prepare(snapshot)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if got, want := len(prepared.Datasets), 7; got != want {
		t.Fatalf("dataset count = %d, want %d", got, want)
	}

	tx := prepared.Datasets["transactions"]
	if got, want := len(tx.Cols), 8; got != want {
		t.Fatalf("transaction columns = %d, want %d", got, want)
	}
	if got, want := len(tx.Rows), 3; got != want {
		t.Fatalf("transaction rows = %d, want %d", got, want)
	}
	assertText := func(row cardgraph.Row, key, want string) {
		t.Helper()
		if got := row[key].Str; got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	assertNumber := func(row cardgraph.Row, key string, want float64) {
		t.Helper()
		if got := row[key].Num; got != want {
			t.Errorf("%s = %v, want %v", key, got, want)
		}
	}
	assertText(tx.Rows[0], "category", "Food")
	assertText(tx.Rows[0], "desc", "Market")
	assertText(tx.Rows[0], "type", "expense")
	assertText(tx.Rows[0], "month", "2026-10")
	assertText(tx.Rows[0], "date", "Oct 10")
	assertNumber(tx.Rows[0], "amount", 12.34)
	assertNumber(tx.Rows[0], "signed", -12.34)
	assertText(tx.Rows[1], "category", "Uncategorized")
	assertText(tx.Rows[1], "desc", "Uncategorized")
	assertText(tx.Rows[1], "type", "income")
	assertNumber(tx.Rows[1], "signed", 50)
	assertText(tx.Rows[2], "type", "transfer")
	assertNumber(tx.Rows[2], "signed", -9)

	if got, want := len(prepared.Datasets["bills"].Rows), 1; got != want {
		t.Fatalf("bill rows = %d, want %d", got, want)
	}
	assertNumber(prepared.Datasets["bills"].Rows[0], "amount", 12.34)
	if got, want := len(prepared.Datasets["accounts"].Rows), 1; got != want {
		t.Fatalf("visible accounts = %d, want %d", got, want)
	}
	assertText(prepared.Datasets["accounts"].Rows[0], "type", "checking")
	assertNumber(prepared.Datasets["budgets"].Rows[0], "limit", 250)
	assertNumber(prepared.Datasets["goals"].Rows[0], "target", 500)
	assertNumber(prepared.Datasets["goals"].Rows[0], "saved", 125)
	assertText(prepared.Datasets["tasks"].Rows[0], "status", "open")

	series := prepared.Datasets["net_worth_series"]
	if got, want := len(series.Cols), 2; got != want {
		t.Fatalf("net worth columns = %d, want %d", got, want)
	}
	if got, want := len(series.Rows), 6; got != want {
		t.Fatalf("net worth series points = %d, want %d", got, want)
	}
	if prepared.NetWorthChange.Base != "USD" || !prepared.NetWorthChange.Known {
		t.Fatalf("net worth change = %#v, want known USD change", prepared.NetWorthChange)
	}
}

func TestPrepareUsesDefaultBaseCurrencyAndPreservesUnavailableDerivedSources(t *testing.T) {
	base := Snapshot{
		At:       time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC),
		Accounts: []domain.Account{{ID: "a", Currency: "USD", OpeningBalance: money.New(100, "USD")}},
	}
	prepared, err := Prepare(base)
	if err != nil {
		t.Fatalf("Prepare default USD: %v", err)
	}
	if prepared.NetWorthChange.Base != "USD" {
		t.Fatalf("base = %q, want USD", prepared.NetWorthChange.Base)
	}

	base.Accounts[0].Currency = "EUR"
	base.Accounts[0].OpeningBalance = money.New(100, "EUR")
	base.Transactions = []domain.Transaction{{ID: "txn", AccountID: "a", Date: base.At, Amount: money.New(-10, "EUR")}}
	prepared, err = Prepare(base)
	if err != nil {
		t.Fatalf("Prepare with missing EUR rate: %v", err)
	}
	if prepared.NetWorthSeriesErr == nil || prepared.NetWorthChangeErr == nil {
		t.Fatalf("missing FX should report unavailable net-worth sources, got series=%v change=%v", prepared.NetWorthSeriesErr, prepared.NetWorthChangeErr)
	}
	if got := len(prepared.Datasets["transactions"].Rows); got != 1 {
		t.Fatalf("transaction rows lost with unavailable net worth = %d, want 1", got)
	}
	if prepared.NetWorthChange.Known {
		t.Fatal("unavailable net-worth change marked known")
	}
}
