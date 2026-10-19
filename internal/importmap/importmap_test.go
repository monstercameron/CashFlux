// SPDX-License-Identifier: MIT

package importmap

import (
	"strings"
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

func TestApplyWithErrorsParsesExactMoneyFormats(t *testing.T) {
	p := Profile{DateCol: 0, DescCol: 1, AmountCol: 2, DebitCol: -1, CreditCol: -1,
		BalanceCol: -1, Decimals: 2, NumberFormat: NumberFormatAuto}
	cases := []struct {
		name   string
		amount string
		want   int64
	}{
		{"dot decimal", "1,234.56", 123456},
		{"comma decimal", "1.234,56", 123456},
		{"detected decimal comma", "12,50", 1250},
		{"sign before currency", "-$12.50", -1250},
		{"accounting negative", "(12.50)", -1250},
		{"accounting negative with currency", "($12.50)", -1250},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows, errs := ApplyWithErrors(p, [][]string{{"2026-06-15", "Test", tc.amount}})
			if len(errs) != 0 {
				t.Fatalf("ApplyWithErrors returned errors: %+v", errs)
			}
			if len(rows) != 1 || rows[0].Amount != tc.want {
				t.Fatalf("amount = %+v, want %d", rows, tc.want)
			}
		})
	}
}

func TestApplyWithErrorsReportsInvalidAmounts(t *testing.T) {
	p := Profile{DateCol: 0, DescCol: 1, AmountCol: 2, DebitCol: -1, CreditCol: -1,
		BalanceCol: -1, Decimals: 2, NumberFormat: NumberFormatDotDecimal}
	rows, errs := ApplyWithErrors(p, [][]string{
		{"2026-06-15", "Bad", "not money"},
		{"2026-06-16", "Good", "3.00"},
	})
	if len(rows) != 1 || rows[0].Amount != 300 {
		t.Fatalf("rows = %+v, want only the valid $3.00 row", rows)
	}
	if len(errs) != 1 || errs[0].Index != 0 || errs[0].Field != "amount" || !strings.Contains(errs[0].Error(), "row 1 amount") {
		t.Fatalf("errors = %+v, want a reported row 1 amount error", errs)
	}
}

func TestApplyWithDiagnosticsDebitSignsAndCheckedSums(t *testing.T) {
	p := Profile{DateCol: 0, DescCol: 1, AmountCol: -1, DebitCol: 2, CreditCol: 3, BalanceCol: -1, Decimals: 2}
	rows, errs, _ := ApplyWithDiagnostics(p, [][]string{
		{"2026-06-15", "positive debit", "45.00", ""},
		{"2026-06-15", "negative debit", "-45.00", ""},
		{"2026-06-15", "overflow", "", "92233720368547758.08"},
	})
	if len(rows) != 2 || rows[0].Amount != -4500 || rows[1].Amount != -4500 {
		t.Fatalf("debit rows = %+v, want both signs mapped to -4500", rows)
	}
	if len(errs) != 1 || errs[0].Index != 2 || errs[0].Field != "credit" {
		t.Fatalf("overflow errors = %+v, want the overflowing credit row", errs)
	}
}

func TestAutoNumberFormatRejectsMalformedAccountingSignAndReadsThreeDigitGroups(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  int64
		bad   bool
	}{{"1,234", 1_234_000, false}, {"($-12.34)", 0, true}} {
		got, err := parseMinorUnits(tc.value, 3, NumberFormatAuto)
		if tc.bad && err == nil {
			t.Errorf("%q unexpectedly parsed as %d", tc.value, got)
		}
		if !tc.bad && (err != nil || got != tc.want) {
			t.Errorf("parse %q = %d, %v; want %d", tc.value, got, err, tc.want)
		}
	}
}

func TestAmbiguousNumericDateIsReported(t *testing.T) {
	p := Profile{DateCol: 0, DescCol: 1, AmountCol: 2, BalanceCol: -1, Decimals: 2}
	rows, errs, warnings := ApplyWithDiagnostics(p, [][]string{{"05/03/2024", "Cafe", "1.00"}})
	if len(errs) != 0 || len(rows) != 1 || len(warnings) != 1 || warnings[0].Index != 0 {
		t.Fatalf("rows=%+v errors=%+v warnings=%+v", rows, errs, warnings)
	}
	if rows[0].Date.Month() != time.May || rows[0].Date.Day() != 3 {
		t.Errorf("ambiguous date parsed as %v, want May 3 with a warning", rows[0].Date)
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

func TestDefaultProfileRecognizesInternationalHeaders(t *testing.T) {
	tests := []struct {
		name   string
		header []string
		date   int
		desc   int
		amount int
		debit  int
		credit int
		bal    int
	}{
		{"Dutch", []string{"Datum", "Buchungstext", "Bedrag", "Saldo"}, 0, 1, 2, -1, -1, 3},
		{"Spanish", []string{"Fecha", "Concepto", "Importe"}, 0, 1, 2, -1, -1, -1},
		{"French", []string{"Date de valeur", "Libellé", "Montant"}, 0, 1, 2, -1, -1, -1},
		{"separate debit and credit", []string{"Fecha", "Concepto", "Débit", "Crédit"}, 0, 1, -1, 2, 3, -1},
		{"signed debit credit column", []string{"Date", "Libellé", "Débit/Crédit"}, 0, 1, 2, -1, -1, -1},
		{"posting date and transaction details", []string{"Posting Date", "Transaction Details", "Ref", "Value"}, 0, 1, 3, -1, -1, -1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DefaultProfile("bank", tc.header, 2)
			if got.DateCol != tc.date || got.DescCol != tc.desc || got.AmountCol != tc.amount || got.DebitCol != tc.debit || got.CreditCol != tc.credit || got.BalanceCol != tc.bal {
				t.Errorf("DefaultProfile mapping = date %d desc %d amount %d debit %d credit %d balance %d, want %d/%d/%d/%d/%d/%d",
					got.DateCol, got.DescCol, got.AmountCol, got.DebitCol, got.CreditCol, got.BalanceCol,
					tc.date, tc.desc, tc.amount, tc.debit, tc.credit, tc.bal)
			}
		})
	}
}
