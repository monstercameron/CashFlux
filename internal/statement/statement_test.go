// SPDX-License-Identifier: MIT

package statement

import (
	"encoding/binary"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestParseDecodesStatementEncodings(t *testing.T) {
	plain := "Date,Description,Amount\n2026-06-01,Caf\u00e9 \u2013 shop,12.34\n"
	utf16Units := utf16.Encode([]rune(plain))
	utf16LE := make([]byte, len(utf16Units)*2)
	for i, unit := range utf16Units {
		binary.LittleEndian.PutUint16(utf16LE[i*2:], unit)
	}
	withBOM := append([]byte{0xFF, 0xFE}, utf16LE...)
	cp1252 := []byte("Date,Description,Amount\n2026-06-01,Caf")
	cp1252 = append(cp1252, 0xE9)
	cp1252 = append(cp1252, []byte(" ")...)
	cp1252 = append(cp1252, 0x96)
	cp1252 = append(cp1252, []byte(" shop,12.34\n")...)
	tests := []struct {
		name string
		data []byte
	}{
		{"UTF-16LE statement with BOM", withBOM},
		{"Windows-1252 statement", cp1252},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(string(tc.data), 2)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Rows) != 1 || got.Rows[0].Amount != 1234 || got.Rows[0].Description != "Caf\u00e9 \u2013 shop" {
				t.Fatalf("rows = %+v, want decoded description and 1234 minor units", got.Rows)
			}
		})
	}
}

func TestDetectDelimiter(t *testing.T) {
	tests := []struct {
		line string
		want rune
	}{
		{"Date,Description,Amount", ','},
		{"Date;Description;Amount", ';'},
		{"Date\tDescription\tAmount", '\t'},
		{"Date|Description|Amount", '|'},
		{"justoneword", ','}, // no delimiter → comma
	}
	for _, tc := range tests {
		if got := DetectDelimiter(tc.line); got != tc.want {
			t.Errorf("DetectDelimiter(%q) = %q, want %q", tc.line, got, tc.want)
		}
	}
}

func TestParseAmount(t *testing.T) {
	tests := []struct {
		in   string
		want int64
	}{
		{"1234.56", 123456},
		{"$1,234.56", 123456},
		{"(1,234.56)", -123456},
		{"-50.00", -5000},
		{"+50.00", 5000},
		{"12.00-", -1200},
		{"+12.00", 1200},
		{"100.00 DR", -10000},
		{"100.00 CR", 10000},
		{"£2,000", 200000},
		{"  12.30  ", 1230},
	}
	for _, tc := range tests {
		got, err := ParseAmount(tc.in, 2)
		if err != nil {
			t.Errorf("ParseAmount(%q) error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseAmount(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
	if _, err := ParseAmount("   ", 2); err == nil {
		t.Error("blank amount should error")
	}
}

func TestParseWithOptionsDetectsNumberFormats(t *testing.T) {
	tests := []struct {
		name string
		csv  string
		want int64
		fmt  NumberFormat
	}{
		{
			name: "US decimal and grouping",
			csv:  "Date,Description,Amount\n2026-01-02,Shop,\"1,234.56\"\n",
			want: 123456,
			fmt:  NumberFormat{DecimalSep: '.', GroupSep: ','},
		},
		{
			name: "European decimal and grouping",
			csv:  "Date;Description;Amount\n2026-01-02;Shop;\"1.234,56\"\n",
			want: 123456,
			fmt:  NumberFormat{DecimalSep: ',', GroupSep: '.'},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			st, err := Parse(tc.csv, 2)
			if err != nil {
				t.Fatal(err)
			}
			if len(st.Rows) != 1 || st.Rows[0].Amount != tc.want {
				t.Fatalf("rows = %+v, want amount %d", st.Rows, tc.want)
			}
			if st.NumberFormat != tc.fmt {
				t.Errorf("format = %+v, want %+v", st.NumberFormat, tc.fmt)
			}
		})
	}
}

func TestParseWithOptionsAmbiguousNumberAndOverride(t *testing.T) {
	csv := "Date,Description,Amount\n2026-01-02,Shop,\"1,234\"\n"
	got, err := Parse(csv, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got.AmbiguousNumbers != 1 || len(got.Rows) != 0 || len(got.Errors) != 1 {
		t.Fatalf("ambiguous result = %+v; want one ambiguity and row error", got)
	}
	got, err = ParseWithOptions(csv, 2, ParseOptions{NumberFormat: NumberFormat{DecimalSep: '.', GroupSep: ','}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Rows) != 1 || got.Rows[0].Amount != 123400 {
		t.Fatalf("override rows = %+v; want 123400 minor units", got.Rows)
	}
}

func TestParseDateOrderDetectionAndOverride(t *testing.T) {
	csv := "Date,Description,Amount\n01/02/2026,First,1.00\n15/02/2026,Second,2.00\n"
	st, err := Parse(csv, 2)
	if err != nil {
		t.Fatal(err)
	}
	if st.DateOrder != DayMonth || st.AmbiguousDates != 1 {
		t.Fatalf("date detection = order %q ambiguous %d", st.DateOrder, st.AmbiguousDates)
	}
	if got := st.Rows[0].Date.Format("2006-01-02"); got != "2026-02-01" {
		t.Errorf("detected DMY date = %s, want 2026-02-01", got)
	}

	ambiguousOnly := "Date,Description,Amount\n01/02/2026,First,1.00\n"
	st, err = ParseWithOptions(ambiguousOnly, 2, ParseOptions{DateOrder: DayMonth})
	if err != nil {
		t.Fatal(err)
	}
	if st.AmbiguousDates != 1 || st.Rows[0].Date.Format("2006-01-02") != "2026-02-01" {
		t.Fatalf("override result = %+v, want DMY and one ambiguity", st)
	}
}

func TestParseDate(t *testing.T) {
	for _, s := range []string{"2026-06-15", "06/15/2026", "6/15/2026", "15 Jun 2026", "Jun 15, 2026"} {
		d, err := ParseDate(s)
		if err != nil {
			t.Errorf("ParseDate(%q) error: %v", s, err)
			continue
		}
		if d.Year() != 2026 || d.Month() != 6 || d.Day() != 15 {
			t.Errorf("ParseDate(%q) = %s, want 2026-06-15", s, d.Format("2006-01-02"))
		}
	}
	if _, err := ParseDate("nonsense"); err == nil {
		t.Error("garbage date should error")
	}
}

func TestMapColumns(t *testing.T) {
	c := MapColumns([]string{"Posted Date", "Memo", "Amount", "Running Balance"})
	if c.Date != 0 || c.Description != 1 || c.Amount != 2 || c.Balance != 3 {
		t.Errorf("MapColumns = %+v, want date0/desc1/amt2/bal3", c)
	}
	dc := MapColumns([]string{"Date", "Description", "Debit", "Credit"})
	if dc.Debit != 2 || dc.Credit != 3 || dc.Amount != -1 {
		t.Errorf("debit/credit map = %+v, want debit2/credit3/amount-1", dc)
	}
}

func TestMapColumnsRecognizesInternationalBankHeaders(t *testing.T) {
	tests := []struct {
		name   string
		header []string
		want   Columns
	}{
		{"Dutch", []string{"Datum", "Buchungstext", "Bedrag"}, Columns{Date: 0, Description: 1, Amount: 2, Debit: -1, Credit: -1, Balance: -1}},
		{"Spanish", []string{"Fecha", "Concepto", "Importe"}, Columns{Date: 0, Description: 1, Amount: 2, Debit: -1, Credit: -1, Balance: -1}},
		{"French", []string{"Date de valeur", "Libellé", "Montant"}, Columns{Date: 0, Description: 1, Amount: 2, Debit: -1, Credit: -1, Balance: -1}},
		{"German unusual names", []string{"Valuta", "Buchungstext", "Betrag", "Saldo"}, Columns{Date: 0, Description: 1, Amount: 2, Debit: -1, Credit: -1, Balance: 3}},
		{"French signed debit credit column", []string{"Date", "Libellé", "Débit/Crédit"}, Columns{Date: 0, Description: 1, Amount: 2, Debit: -1, Credit: -1, Balance: -1}},
		{"separate debit and credit", []string{"Fecha", "Concepto", "Débit", "Crédit"}, Columns{Date: 0, Description: 1, Amount: -1, Debit: 2, Credit: 3, Balance: -1}},
		{"posting date and transaction details", []string{"Posting Date", "Transaction Details", "Ref", "Value"}, Columns{Date: 0, Description: 1, Amount: 3, Debit: -1, Credit: -1, Balance: -1}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := MapColumns(tc.header); got != tc.want {
				t.Errorf("MapColumns(%q) = %+v, want %+v", tc.header, got, tc.want)
			}
		})
	}
}

func TestParseRecognizesInternationalAndSeparateDebitCreditHeaders(t *testing.T) {
	for _, tc := range []struct {
		name string
		csv  string
		want int64
	}{
		{"Dutch", "Datum;Buchungstext;Betrag\n2026-06-15;Einkauf;-12,50\n", -1250},
		{"Spanish", "Fecha;Concepto;Importe\n2026-06-16;Compra;-8,25\n", -825},
		{"French debit credit", "Date;Libellé;Débit;Crédit\n2026-06-17;Achat;12,50;\n", -1250},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st, err := Parse(tc.csv, 2)
			if err != nil {
				t.Fatal(err)
			}
			if len(st.Rows) != 1 || st.Rows[0].Amount != tc.want {
				t.Fatalf("rows = %+v, want one row amount %d", st.Rows, tc.want)
			}
		})
	}
}

func TestParseSingleAmountColumn(t *testing.T) {
	csv := "Date,Description,Amount,Balance\n" +
		"2026-06-01,Coffee Shop,(4.50),100.00\n" +
		"2026-06-02,Paycheck,\"2,000.00\",2100.00\n"
	st, err := Parse(csv, 2)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(st.Rows) != 2 || len(st.Errors) != 0 {
		t.Fatalf("rows=%d errors=%d, want 2/0 (%+v)", len(st.Rows), len(st.Errors), st.Errors)
	}
	if st.Rows[0].Amount != -450 || st.Rows[0].Description != "Coffee Shop" {
		t.Errorf("row0 = %+v, want -450 / Coffee Shop", st.Rows[0])
	}
	if st.Rows[1].Amount != 200000 || !st.Rows[1].HasBalance || st.Rows[1].Balance != 210000 {
		t.Errorf("row1 = %+v, want 200000 amount / balance 210000", st.Rows[1])
	}
}

func TestParseDebitCreditColumns(t *testing.T) {
	data := "Date;Description;Debit;Credit\n" +
		"15/06/2026;ATM Withdrawal;40.00;\n" +
		"16/06/2026;Refund;;25.00\n"
	st, err := Parse(data, 2)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if st.Delimiter != ';' {
		t.Errorf("delimiter = %q, want ';'", st.Delimiter)
	}
	if len(st.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(st.Rows))
	}
	if st.Rows[0].Amount != -4000 {
		t.Errorf("debit row = %d, want -4000", st.Rows[0].Amount)
	}
	if st.Rows[1].Amount != 2500 {
		t.Errorf("credit row = %d, want 2500", st.Rows[1].Amount)
	}
}

func TestParseRowErrorsAreSkippedNotFatal(t *testing.T) {
	csv := "Date,Description,Amount\n" +
		"2026-06-01,Good,10.00\n" +
		"not-a-date,Bad,5.00\n" +
		"2026-06-03,AlsoGood,20.00\n"
	st, err := Parse(csv, 2)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(st.Rows) != 2 {
		t.Errorf("good rows = %d, want 2", len(st.Rows))
	}
	if len(st.Errors) != 1 || st.Errors[0].Line != 3 {
		t.Errorf("errors = %+v, want one at line 3", st.Errors)
	}
}

func TestParseRejectsUnmappableHeader(t *testing.T) {
	if _, err := Parse("Foo,Bar\n1,2\n", 2); err == nil {
		t.Error("a header with no date/amount columns should error")
	} else if !strings.Contains(err.Error(), "map the columns") {
		t.Errorf("unmappable header error = %q, want mapping guidance", err)
	}
}
