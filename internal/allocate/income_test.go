// SPDX-License-Identifier: MIT

package allocate

import (
	"errors"
	"testing"
	"time"
)

func d(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

// inc builds an income Transaction.
func inc(amount int64, cur, date string) Transaction {
	return Transaction{Amount: amount, Currency: cur, IsIncome: true, Date: d(date)}
}

// exp builds an expense Transaction (IsIncome=false).
func exp(amount int64, cur, date string) Transaction {
	return Transaction{Amount: -amount, Currency: cur, IsIncome: false, Date: d(date)}
}

func TestPeriodIncomeBasic(t *testing.T) {
	txns := []Transaction{
		inc(100000, "USD", "2026-06-05"), // 1000.00 USD income
		inc(50000, "USD", "2026-06-20"),  // 500.00  USD income
		exp(30000, "USD", "2026-06-10"),  // expense — excluded
	}
	start, end := d("2026-06-01"), d("2026-07-01")
	got, err := PeriodIncome(txns, start, end, "USD", NoConvert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 150000 {
		t.Errorf("PeriodIncome = %d, want 150000", got)
	}
}

func TestPeriodIncomeExcludesOutsideRange(t *testing.T) {
	txns := []Transaction{
		inc(100000, "USD", "2026-05-31"), // before start — excluded
		inc(50000, "USD", "2026-06-01"),  // on start — included
		inc(20000, "USD", "2026-06-30"),  // last day — included
		inc(10000, "USD", "2026-07-01"),  // on end (exclusive) — excluded
	}
	start, end := d("2026-06-01"), d("2026-07-01")
	got, err := PeriodIncome(txns, start, end, "USD", NoConvert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 70000 {
		t.Errorf("PeriodIncome = %d, want 70000 (only Jun 1 + Jun 30)", got)
	}
}

func TestPeriodIncomeOnlyIncome(t *testing.T) {
	txns := []Transaction{
		{Amount: -5000, Currency: "USD", IsIncome: false, Date: d("2026-06-10")},
		{Amount: 8000, Currency: "USD", IsIncome: false, Date: d("2026-06-12")}, // positive but not income
	}
	start, end := d("2026-06-01"), d("2026-07-01")
	got, err := PeriodIncome(txns, start, end, "USD", NoConvert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 0 {
		t.Errorf("PeriodIncome = %d, want 0 (no income transactions)", got)
	}
}

func TestPeriodIncomeEmpty(t *testing.T) {
	got, err := PeriodIncome(nil, d("2026-06-01"), d("2026-07-01"), "USD", NoConvert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 0 {
		t.Errorf("PeriodIncome = %d, want 0 for empty slice", got)
	}
}

func TestPeriodIncomeConverts(t *testing.T) {
	// EUR transactions converted to USD at 1.1 rate.
	convert := func(amount int64, from, to string) (int64, error) {
		if from == to {
			return amount, nil
		}
		if from == "EUR" && to == "USD" {
			return int64(float64(amount) * 1.1), nil
		}
		return 0, errors.New("no rate")
	}
	txns := []Transaction{
		inc(10000, "EUR", "2026-06-15"), // 10000 EUR → 11000 USD
		inc(5000, "USD", "2026-06-20"),  // 5000 USD
	}
	start, end := d("2026-06-01"), d("2026-07-01")
	got, err := PeriodIncome(txns, start, end, "USD", convert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 16000 {
		t.Errorf("PeriodIncome = %d, want 16000 (11000+5000)", got)
	}
}

func TestPeriodIncomeConverterErrorPropagates(t *testing.T) {
	boom := func(int64, string, string) (int64, error) { return 0, errors.New("no rate for XYZ") }
	txns := []Transaction{inc(100, "XYZ", "2026-06-10")}
	_, err := PeriodIncome(txns, d("2026-06-01"), d("2026-07-01"), "USD", boom)
	if err == nil {
		t.Error("expected error from failing converter, got nil")
	}
}

func TestPeriodIncomeNegativeTotalClampsToZero(t *testing.T) {
	// Contrived: income transactions with negative amounts (e.g. reversed paychecks).
	txns := []Transaction{
		{Amount: -5000, Currency: "USD", IsIncome: true, Date: d("2026-06-10")},
	}
	got, err := PeriodIncome(txns, d("2026-06-01"), d("2026-07-01"), "USD", NoConvert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 0 {
		t.Errorf("PeriodIncome = %d, want 0 (negative clamped)", got)
	}
}

func TestPeriodIncomeMultipleMonths(t *testing.T) {
	txns := []Transaction{
		inc(30000, "USD", "2026-05-15"), // May — excluded
		inc(50000, "USD", "2026-06-01"), // June — included
		inc(70000, "USD", "2026-07-01"), // July — excluded (end exclusive)
	}
	start, end := d("2026-06-01"), d("2026-07-01")
	got, err := PeriodIncome(txns, start, end, "USD", NoConvert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 50000 {
		t.Errorf("PeriodIncome = %d, want 50000 (only June txn)", got)
	}
}
