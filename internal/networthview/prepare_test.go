// SPDX-License-Identifier: MIT

package networthview

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestPreparePreservesWindowAndExpenseBoundaries(t *testing.T) {
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	account := domain.Account{ID: "cash", Currency: "USD", Type: domain.TypeChecking, OpeningBalance: money.New(1000, "USD")}
	txns := []domain.Transaction{
		{AccountID: "cash", Date: time.Date(2025, 11, 1, 0, 0, 0, 0, time.UTC), Amount: money.New(100, "USD")},
		{AccountID: "cash", Date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Amount: money.New(-600, "USD")},
		{AccountID: "cash", Date: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), Amount: money.New(-900, "USD")},
		{AccountID: "cash", Date: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), Amount: money.New(-1500, "USD")},
		{AccountID: "cash", Date: now, Amount: money.New(50, "USD")},
		{AccountID: "cash", Date: time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC), Amount: money.New(999, "USD")},
	}
	accounts := []domain.Account{account}
	accountsBefore, txnsBefore := append([]domain.Account(nil), accounts...), append([]domain.Transaction(nil), txns...)
	got, err := Prepare(Input{Accounts: accounts, Transactions: txns, Rates: currency.Rates{Base: "USD"}, Months: 6, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Since.Equal(time.Date(2025, 11, 1, 0, 0, 0, 0, time.UTC)) || !got.Until.Equal(time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("wrong half-open window: [%v,%v)", got.Since, got.Until)
	}
	if !got.Snapshot.Known || got.Snapshot.Net.Amount != -851 {
		t.Fatalf("snapshot did not include future transaction: %+v", got.Snapshot)
	}
	if got.SeriesErr != nil || len(got.Points) != len(got.Cutoffs) {
		t.Fatalf("series unavailable: %v, points=%d cutoffs=%d", got.SeriesErr, len(got.Points), len(got.Cutoffs))
	}
	last := got.Points[len(got.Points)-1]
	if last.NetMinor != -1850 {
		t.Fatalf("until cutoff must include today and exclude next-day entry: %+v", last)
	}
	if !got.Expense.Known || got.Expense.Err != nil || got.Expense.Amount.Amount != 1000 || got.Expense.Amount.Currency != "USD" {
		t.Fatalf("three complete-month average: %+v", got.Expense)
	}
	if got.HealthErr != nil || !got.HealthKnown || got.RunwayKnown || !got.CashKnown {
		t.Fatalf("health facts unavailable or misleading: %+v", got)
	}
	if !reflect.DeepEqual(accountsBefore, accounts) || !reflect.DeepEqual(txnsBefore, txns) {
		t.Fatal("prepare mutated inputs")
	}
}

func TestPrepareKeepsEngineAvailabilityIndependent(t *testing.T) {
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	accounts := []domain.Account{
		{ID: "cash", Currency: "USD", Type: domain.TypeChecking, OpeningBalance: money.New(1000, "USD")},
		{ID: "foreign", Currency: "EUR", OpeningBalance: money.New(200, "EUR")},
	}
	txns := []domain.Transaction{{AccountID: "foreign", Date: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), Amount: money.New(-100, "EUR")}}
	got, err := Prepare(Input{Accounts: accounts, Transactions: txns, Rates: currency.Rates{Base: "USD"}, Months: 6, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Snapshot.Known || !got.Snapshot.Partial || got.SnapshotErr != nil {
		t.Fatalf("snapshot availability lost: %+v", got.Snapshot)
	}
	if got.SeriesErr == nil || got.BridgeErr == nil {
		t.Fatalf("missing FX did not invalidate affected engines: series=%v bridge=%v", got.SeriesErr, got.BridgeErr)
	}
	if got.Expense.Known || got.Expense.Err == nil {
		t.Fatalf("missing FX was presented as zero spend: %+v", got.Expense)
	}
	if got.CashKnown || got.HealthKnown || got.RunwayKnown {
		t.Fatalf("derived composition survived unavailable series: %+v", got)
	}
}

func TestPrepareRejectsUnsupportedWindowAndChecksHealthArithmetic(t *testing.T) {
	now := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	if _, err := Prepare(Input{Rates: currency.Rates{Base: "USD"}, Months: 2, Now: now}); !errors.Is(err, ErrInvalidWindow) {
		t.Fatalf("unsupported months error = %v", err)
	}
	if _, err := Prepare(Input{Rates: currency.Rates{Base: "USD"}, Months: 6}); !errors.Is(err, ErrInvalidWindow) {
		t.Fatalf("missing reference date error = %v", err)
	}
	if _, err := assessChecked(100, 100, math.MaxInt64/2+1, 1); err == nil {
		t.Fatal("overflowing liquid-share arithmetic became known health")
	}
	if _, err := assessChecked(100, 100, 1, 1); err != nil {
		t.Fatalf("ordinary health ratios failed: %v", err)
	}
}

func TestPrepareAllTimeWindowUsesEarliestTransactionMonth(t *testing.T) {
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	accounts := []domain.Account{{ID: "cash", Currency: "USD", OpeningBalance: money.New(500, "USD")}}
	txns := []domain.Transaction{{AccountID: "cash", Date: time.Date(2021, 7, 17, 0, 0, 0, 0, time.UTC), Amount: money.New(5, "USD")}}
	got, err := Prepare(Input{Accounts: accounts, Transactions: txns, Rates: currency.Rates{Base: "USD"}, Months: 0, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Since.Equal(time.Date(2021, 7, 1, 0, 0, 0, 0, time.UTC)) || !got.Until.Equal(time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("wrong all-time window: [%v,%v)", got.Since, got.Until)
	}
	if len(got.Cutoffs) > 38 {
		t.Fatalf("all-time history was not thinned: %d cutoffs", len(got.Cutoffs))
	}
}

func TestPreparePartitionsAndOrdersAccountFacts(t *testing.T) {
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	accounts := []domain.Account{
		{ID: "small", Currency: "USD", Class: domain.ClassAsset, OpeningBalance: money.New(100, "USD")},
		{ID: "large", Currency: "USD", Class: domain.ClassAsset, OpeningBalance: money.New(500, "USD")},
		{ID: "debt", Currency: "USD", Class: domain.ClassLiability, OpeningBalance: money.New(-300, "USD")},
		{ID: "archived", Currency: "USD", Class: domain.ClassAsset, Archived: true, OpeningBalance: money.New(900, "USD")},
	}
	txns := []domain.Transaction{
		{AccountID: "small", Date: time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC), Amount: money.New(-10, "USD")},
		{AccountID: "large", Date: time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC), Amount: money.New(10, "USD")},
	}
	got, err := Prepare(Input{Accounts: accounts, Transactions: txns, Rates: currency.Rates{Base: "USD"}, Months: 1, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Accounts) != 3 || got.Accounts[0].Account.ID != "large" || got.Accounts[1].Account.ID != "debt" || got.Accounts[2].Account.ID != "small" {
		t.Fatalf("all-account magnitude order = %+v", got.Accounts)
	}
	if len(got.Assets) != 2 || got.Assets[0].Account.ID != "large" || len(got.Liabilities) != 1 || got.Liabilities[0].Account.ID != "debt" {
		t.Fatalf("side partitions mismatch: assets=%+v liabilities=%+v", got.Assets, got.Liabilities)
	}
	if len(got.Movers) != 2 || got.Movers[0].Account.ID != "large" || got.Movers[1].Account.ID != "small" {
		t.Fatalf("mover order / ties mismatch: %+v", got.Movers)
	}
}
