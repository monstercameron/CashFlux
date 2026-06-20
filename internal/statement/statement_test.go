package statement

import "testing"

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
	}
}
