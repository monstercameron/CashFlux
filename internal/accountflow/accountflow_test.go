// SPDX-License-Identifier: MIT

package accountflow

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func acct(id string, opening int64) domain.Account {
	return domain.Account{ID: id, Currency: "USD", OpeningBalance: money.New(opening, "USD")}
}

func txn(id, acctID string, amount int64, when time.Time) domain.Transaction {
	return domain.Transaction{ID: id, AccountID: acctID, Amount: money.New(amount, "USD"), Date: when}
}

func TestPeriodFlow(t *testing.T) {
	a := acct("chk", 0)
	start, end := day(2026, 7, 1), day(2026, 8, 1)
	txns := []domain.Transaction{
		txn("i1", "chk", 200000, day(2026, 7, 3)),    // income +2000
		txn("e1", "chk", -50000, day(2026, 7, 10)),   // spend 500
		txn("e2", "chk", -25000, day(2026, 7, 15)),   // spend 250
		txn("old", "chk", -99900, day(2026, 6, 30)),  // outside period
		txn("other", "sav", -10000, day(2026, 7, 5)), // other account
	}
	// A transfer OUT should not count as spending.
	tr := txn("t1", "chk", -100000, day(2026, 7, 20))
	tr.TransferAccountID = "sav"
	txns = append(txns, tr)

	f := PeriodFlow(a, txns, start, end)
	if f.In.Amount != 200000 {
		t.Errorf("In = %d, want 200000", f.In.Amount)
	}
	if f.Out.Amount != 75000 {
		t.Errorf("Out = %d, want 75000", f.Out.Amount)
	}
	if f.Net.Amount != 125000 {
		t.Errorf("Net = %d, want 125000", f.Net.Amount)
	}
	if f.Transfer.Amount != -100000 {
		t.Errorf("Transfer = %d, want -100000", f.Transfer.Amount)
	}
}

func TestPeriodFlowEmpty(t *testing.T) {
	f := PeriodFlow(acct("chk", 0), nil, day(2026, 7, 1), day(2026, 8, 1))
	if f.In.Amount != 0 || f.Out.Amount != 0 || f.Net.Amount != 0 {
		t.Errorf("empty flow = %+v, want zeros", f)
	}
	if f.Net.Currency != "USD" {
		t.Errorf("Net currency = %q, want USD", f.Net.Currency)
	}
}

func TestBalanceSeries(t *testing.T) {
	a := acct("chk", 100000) // opening 1000
	txns := []domain.Transaction{
		txn("a", "chk", 50000, day(2026, 7, 2)),  // +500 -> 1500
		txn("b", "chk", -20000, day(2026, 7, 4)), // -200 -> 1300
	}
	// 5-day window ending Jul 5: days Jul 1..Jul 5.
	series := BalanceSeries(a, txns, day(2026, 7, 5), 5)
	want := []int64{100000, 150000, 150000, 130000, 130000}
	if len(series) != 5 {
		t.Fatalf("len = %d, want 5", len(series))
	}
	for i := range want {
		if series[i] != want[i] {
			t.Errorf("series[%d] = %d, want %d", i, series[i], want[i])
		}
	}
}

func TestBalanceSeriesFlat(t *testing.T) {
	a := acct("chk", 42000)
	series := BalanceSeries(a, nil, day(2026, 7, 30), 90)
	if len(series) != 90 {
		t.Fatalf("len = %d, want 90", len(series))
	}
	for i, v := range series {
		if v != 42000 {
			t.Fatalf("series[%d] = %d, want flat 42000", i, v)
		}
	}
}

func TestBalanceSeriesZeroDays(t *testing.T) {
	if s := BalanceSeries(acct("chk", 0), nil, day(2026, 7, 1), 0); s != nil {
		t.Errorf("days=0 = %v, want nil", s)
	}
}

func TestPolyline(t *testing.T) {
	// Ascending series maps to descending y (SVG y grows downward).
	pts := Polyline([]int64{0, 100}, 100, 20, 2)
	if pts != "0.00,18.00 100.00,2.00" {
		t.Errorf("Polyline = %q", pts)
	}
	// Flat series centers vertically.
	flat := Polyline([]int64{5, 5, 5}, 100, 20, 2)
	if flat != "0.00,10.00 50.00,10.00 100.00,10.00" {
		t.Errorf("flat Polyline = %q", flat)
	}
	// Too few points draws nothing.
	if Polyline([]int64{1}, 100, 20, 2) != "" {
		t.Error("single-point Polyline should be empty")
	}
}
