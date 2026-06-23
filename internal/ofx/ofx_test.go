package ofx

import (
	"strings"
	"testing"
	"time"
)

func TestParse_SGML(t *testing.T) {
	input := `OFXHEADER:100
DATA:OFXSGML
VERSION:102

<OFX>
<BANKMSGSRSV1>
<STMTTRNRS>
<STMTRS>
<BANKTRANLIST>
<STMTTRN>
<TRNTYPE>DEBIT
<DTPOSTED>20260615120000[0:GMT]
<TRNAMT>-45.00
<FITID>202606151
<NAME>Coffee Shop
</STMTTRN>
<STMTTRN>
<TRNTYPE>CREDIT
<DTPOSTED>20260620
<TRNAMT>1200.00
<FITID>202606202
<NAME>Payroll
</STMTTRN>
</BANKTRANLIST>
</STMTRS>
</STMTTRNRS>
</BANKMSGSRSV1>
</OFX>`

	rows, err := Parse(strings.NewReader(input), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	tests := []struct {
		desc   string
		amount int64
		fitid  string
		date   time.Time
	}{
		{"Coffee Shop", -4500, "202606151", time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)},
		{"Payroll", 120000, "202606202", time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)},
	}
	for i, tc := range tests {
		r := rows[i]
		if r.Description != tc.desc {
			t.Errorf("row %d desc: got %q want %q", i, r.Description, tc.desc)
		}
		if r.Amount != tc.amount {
			t.Errorf("row %d amount: got %d want %d", i, r.Amount, tc.amount)
		}
		if r.FITID != tc.fitid {
			t.Errorf("row %d fitid: got %q want %q", i, r.FITID, tc.fitid)
		}
		if !r.Date.Equal(tc.date) {
			t.Errorf("row %d date: got %v want %v", i, r.Date, tc.date)
		}
	}
}

func TestParse_XML(t *testing.T) {
	input := `<OFX>
<BANKMSGSRSV1>
<STMTTRNRS>
<STMTRS>
<BANKTRANLIST>
<STMTTRN>
<DTPOSTED>20260615120000</DTPOSTED>
<TRNAMT>-45.00</TRNAMT>
<FITID>TX001</FITID>
<NAME>Coffee Shop</NAME>
</STMTTRN>
<STMTTRN>
<DTPOSTED>20260620</DTPOSTED>
<TRNAMT>1200.00</TRNAMT>
<FITID>TX002</FITID>
<NAME>Payroll</NAME>
</STMTTRN>
</BANKTRANLIST>
</STMTRS>
</STMTTRNRS>
</BANKMSGSRSV1>
</OFX>`

	rows, err := Parse(strings.NewReader(input), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Amount != -4500 {
		t.Errorf("row 0 amount: got %d want -4500", rows[0].Amount)
	}
	if rows[1].Amount != 120000 {
		t.Errorf("row 1 amount: got %d want 120000", rows[1].Amount)
	}
}

func TestParse_UnrecognizedFormat(t *testing.T) {
	_, err := Parse(strings.NewReader("hello world"), 2)
	if err == nil {
		t.Fatal("expected error for unrecognized format")
	}
}

func TestParse_ZeroDecimals(t *testing.T) {
	input := `OFXHEADER:100
DATA:OFXSGML
<OFX>
<STMTTRN>
<DTPOSTED>20260101
<TRNAMT>-45
<FITID>1
<NAME>Test
</STMTTRN>
</OFX>`
	rows, err := Parse(strings.NewReader(input), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Amount != -45 {
		t.Errorf("amount: got %d want -45", rows[0].Amount)
	}
}
