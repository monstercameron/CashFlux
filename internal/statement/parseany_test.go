package statement

import (
	"strings"
	"testing"
)

func TestParseAny_OFX1(t *testing.T) {
	input := `OFXHEADER:100
DATA:OFXSGML
VERSION:102

<OFX>
<BANKMSGSRSV1>
<STMTTRNRS>
<STMTRS>
<BANKTRANLIST>
<STMTTRN>
<DTPOSTED>20260615
<TRNAMT>-45.00
<FITID>1
<NAME>Coffee Shop
</STMTTRN>
</BANKTRANLIST>
</STMTRS>
</STMTTRNRS>
</BANKMSGSRSV1>
</OFX>`

	rows, err := ParseAny(strings.NewReader(input), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Amount != -4500 {
		t.Errorf("amount: got %d want -4500", rows[0].Amount)
	}
	if rows[0].Description != "Coffee Shop" {
		t.Errorf("description: got %q want %q", rows[0].Description, "Coffee Shop")
	}
}

func TestParseAny_CSV(t *testing.T) {
	input := "Date,Description,Amount\n2026-06-15,Coffee Shop,-45.00\n2026-06-20,Payroll,1200.00"
	rows, err := ParseAny(strings.NewReader(input), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The CSV parser (statement.Parse) handles this
	if len(rows) == 0 {
		t.Fatal("expected at least 1 row from CSV")
	}
}
