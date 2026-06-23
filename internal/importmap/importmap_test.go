package importmap

import (
	"testing"
	"time"
)

func TestApply_AmountCol(t *testing.T) {
	p := Profile{
		DateCol:     0,
		DescCol:     1,
		AmountCol:   2,
		DebitCol:    -1,
		CreditCol:   -1,
		BalanceCol:  -1,
		CurrencyCol: -1,
		Decimals:    2,
	}
	rows := [][]string{
		{"2026-06-15", "Coffee Shop", "-45.00"},
		{"2026-06-20", "Payroll", "1200.00"},
	}
	got := Apply(p, rows)
	if len(got) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(got))
	}
	if got[0].Amount != -4500 {
		t.Errorf("row 0 amount: got %d want -4500", got[0].Amount)
	}
	if got[1].Amount != 120000 {
		t.Errorf("row 1 amount: got %d want 120000", got[1].Amount)
	}
}

func TestApply_DebitCredit(t *testing.T) {
	p := Profile{
		DateCol:     0,
		DescCol:     1,
		AmountCol:   -1,
		DebitCol:    2,
		CreditCol:   3,
		BalanceCol:  -1,
		CurrencyCol: -1,
		Decimals:    2,
	}
	rows := [][]string{
		{"2026-06-15", "Coffee Shop", "45.00", ""},
		{"2026-06-20", "Payroll", "", "1200.00"},
	}
	got := Apply(p, rows)
	if len(got) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(got))
	}
	if got[0].Amount != -4500 {
		t.Errorf("row 0 amount: got %d want -4500", got[0].Amount)
	}
	if got[1].Amount != 120000 {
		t.Errorf("row 1 amount: got %d want 120000", got[1].Amount)
	}
}

func TestApply_Balance(t *testing.T) {
	p := Profile{
		DateCol:     0,
		DescCol:     1,
		AmountCol:   2,
		DebitCol:    -1,
		CreditCol:   -1,
		BalanceCol:  3,
		CurrencyCol: -1,
		Decimals:    2,
	}
	rows := [][]string{
		{"2026-06-15", "Test", "-10.00", "990.00"},
	}
	got := Apply(p, rows)
	if len(got) != 1 {
		t.Fatalf("expected 1 row, got %d", len(got))
	}
	if !got[0].HasBalance {
		t.Error("expected HasBalance=true")
	}
	if got[0].Balance != 99000 {
		t.Errorf("balance: got %d want 99000", got[0].Balance)
	}
}

func TestApply_SkipsBadRows(t *testing.T) {
	p := Profile{
		DateCol:     0,
		DescCol:     1,
		AmountCol:   2,
		DebitCol:    -1,
		CreditCol:   -1,
		BalanceCol:  -1,
		CurrencyCol: -1,
		Decimals:    2,
	}
	rows := [][]string{
		{"not-a-date", "Bad row", "10.00"},
		{"2026-06-15", "Good row", "10.00"},
	}
	got := Apply(p, rows)
	if len(got) != 1 {
		t.Fatalf("expected 1 row, got %d", len(got))
	}
	if got[0].Description != "Good row" {
		t.Errorf("unexpected description: %q", got[0].Description)
	}
}

func TestApply_CustomDateLayout(t *testing.T) {
	p := Profile{
		DateCol:     0,
		DescCol:     1,
		AmountCol:   2,
		DebitCol:    -1,
		CreditCol:   -1,
		BalanceCol:  -1,
		CurrencyCol: -1,
		Decimals:    2,
		DateLayout:  "01/02/2006",
	}
	rows := [][]string{
		{"06/15/2026", "Test", "-5.00"},
	}
	got := Apply(p, rows)
	if len(got) != 1 {
		t.Fatalf("expected 1 row, got %d", len(got))
	}
	want := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	if !got[0].Date.Equal(want) {
		t.Errorf("date: got %v want %v", got[0].Date, want)
	}
}

func TestDefaultProfile(t *testing.T) {
	tests := []struct {
		header   []string
		wantDate int
		wantDesc int
		wantAmt  int
	}{
		{
			[]string{"Date", "Description", "Amount"},
			0, 1, 2,
		},
		{
			[]string{"Posted Date", "Memo", "Debit", "Credit"},
			0, 1, -1,
		},
		{
			[]string{"Transaction Date", "Payee", "Amount", "Balance"},
			0, 1, 2,
		},
	}
	for _, tc := range tests {
		p := DefaultProfile("test", tc.header, 2)
		if p.DateCol != tc.wantDate {
			t.Errorf("header %v: DateCol got %d want %d", tc.header, p.DateCol, tc.wantDate)
		}
		if p.DescCol != tc.wantDesc {
			t.Errorf("header %v: DescCol got %d want %d", tc.header, p.DescCol, tc.wantDesc)
		}
		if p.AmountCol != tc.wantAmt {
			t.Errorf("header %v: AmountCol got %d want %d", tc.header, p.AmountCol, tc.wantAmt)
		}
	}
}
