// SPDX-License-Identifier: MIT

package txncalendar

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
)

func TestMonthCashflowCountsVisibleTransferPairOnce(t *testing.T) {
	date := time.Date(2026, time.October, 12, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		{ID: "out", Date: date, Amount: money.New(-50000, "USD"), AccountID: "checking", TransferAccountID: "savings", TransferGroupID: "sweep"},
		{ID: "in", Date: date, Amount: money.New(50000, "USD"), AccountID: "savings", TransferAccountID: "checking", TransferGroupID: "sweep"},
	}
	countedRows := txnclassify.SummarizeVisibleTransfers(txns).CountedRows
	var countLineMoved int64
	for i, txn := range txns {
		if !countedRows[i] {
			continue
		}
		magnitude, err := money.Abs(txn.Amount.Amount)
		if err != nil {
			t.Fatal(err)
		}
		countLineMoved, err = money.Add(countLineMoved, magnitude)
		if err != nil {
			t.Fatal(err)
		}
	}

	got, err := MonthCashflow(date, time.Sunday, txns, nil, currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatalf("MonthCashflow: %v", err)
	}
	var dayMoved, weekMoved int64
	for _, week := range got.Weeks {
		for _, day := range week.Days {
			if DayKey(day.Date) == "2026-10-12" {
				dayMoved = day.Cashflow.Moved
				weekMoved = week.Total.Moved
			}
		}
	}
	if dayMoved != 50000 {
		t.Errorf("day moved = %d, want 50000 cents", dayMoved)
	}
	if weekMoved != countLineMoved || weekMoved != 50000 {
		t.Errorf("week moved = %d; count line = %d; want both 50000 cents", weekMoved, countLineMoved)
	}
}

func TestMonthCashflowExcludesReportRowsFromSpentAndIncome(t *testing.T) {
	date := time.Date(2026, time.October, 12, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		{ID: "included-expense", Date: date, Amount: money.New(-10000, "USD")},
		{ID: "excluded-expense", Date: date, Amount: money.New(-20000, "USD"), ExcludeFromReports: true},
		{ID: "included-income", Date: date, Amount: money.New(30000, "USD")},
		{ID: "excluded-income", Date: date, Amount: money.New(40000, "USD"), ExcludeFromReports: true},
	}
	got, err := MonthCashflow(date, time.Sunday, txns, nil, currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatalf("MonthCashflow: %v", err)
	}
	for _, week := range got.Weeks {
		for _, day := range week.Days {
			if DayKey(day.Date) == "2026-10-12" {
				if day.Cashflow.Spent != 10000 || day.Cashflow.Income != 30000 || day.Cashflow.Net != 20000 {
					t.Errorf("cashflow = %+v, want spent 10000, income 30000, net 20000", day.Cashflow)
				}
				return
			}
		}
	}
	t.Fatal("October 12 cell missing")
}

func TestMonthCashflowDoesNotCountExcludedTransfersAsMoved(t *testing.T) {
	date := time.Date(2026, time.October, 12, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		{ID: "out", Date: date, Amount: money.New(-50000, "USD"), AccountID: "checking", TransferAccountID: "savings", TransferGroupID: "excluded", ExcludeFromReports: true},
		{ID: "in", Date: date, Amount: money.New(50000, "USD"), AccountID: "savings", TransferAccountID: "checking", TransferGroupID: "excluded", ExcludeFromReports: true},
	}
	got, err := MonthCashflow(date, time.Sunday, txns, nil, currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatalf("MonthCashflow: %v", err)
	}
	day := cashflowDay(t, got, date)
	if day.Count != 2 || day.Moved != 0 {
		t.Fatalf("excluded transfer day = %+v, want two visible rows and no moved amount", day)
	}
}

func TestMonthCashflowDoesNotReportRateIssueForUncountedTransferLeg(t *testing.T) {
	date := time.Date(2026, time.October, 12, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		{ID: "unknown-in", Date: date, Amount: money.New(50000, "EUR"), AccountID: "savings", TransferAccountID: "checking", TransferGroupID: "paired"},
		{ID: "out", Date: date, Amount: money.New(-50000, "USD"), AccountID: "checking", TransferAccountID: "savings", TransferGroupID: "paired"},
	}
	got, err := MonthCashflow(date, time.Sunday, txns, nil, currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatalf("MonthCashflow: %v", err)
	}
	if len(got.RateIssues) != 0 {
		t.Fatalf("rate issues = %+v, want none for the hidden partner leg", got.RateIssues)
	}
	day := cashflowDay(t, got, date)
	if day.Moved != 50000 || day.Count != 2 {
		t.Fatalf("paired transfer day = %+v, want one $500.00 movement and both visible rows counted", day)
	}
}

func TestMonthCashflowPairsLegacyTransfersByConvertedAmount(t *testing.T) {
	date := time.Date(2026, time.October, 12, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		{ID: "out", Date: date, Amount: money.New(-10000, "EUR"), AccountID: "checking", TransferAccountID: "savings"},
		{ID: "in", Date: date.AddDate(0, 0, 1), Amount: money.New(12500, "USD"), AccountID: "savings", TransferAccountID: "checking"},
	}
	got, err := MonthCashflow(date, time.Sunday, txns, nil, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.25}})
	if err != nil {
		t.Fatalf("MonthCashflow: %v", err)
	}
	if len(got.RateIssues) != 0 {
		t.Fatalf("rate issues = %+v, want none", got.RateIssues)
	}
	if moved := cashflowDay(t, got, date).Moved; moved != 12500 {
		t.Fatalf("moved = %d, want converted pair counted once as 12500 base minor units", moved)
	}
}

func cashflowDay(t *testing.T, month CashflowMonth, date time.Time) Cashflow {
	t.Helper()
	key := DayKey(date)
	for _, week := range month.Weeks {
		for _, day := range week.Days {
			if DayKey(day.Date) == key {
				return day.Cashflow
			}
		}
	}
	t.Fatalf("calendar cell %s is missing", key)
	return Cashflow{}
}
