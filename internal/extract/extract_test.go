// SPDX-License-Identifier: MIT

package extract

import "testing"

func TestParseRowsArray(t *testing.T) {
	in := `[{"date":"2026-06-01","description":"Coffee","amount":-4.5,"category":"Food"},
	       {"date":"2026-06-02","merchant":"Payday","amount":"2500.00"}]`
	rows, err := ParseRows(in)
	if err != nil {
		t.Fatalf("ParseRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Date != "2026-06-01" || rows[0].Description != "Coffee" || rows[0].Amount != "-4.5" || rows[0].Category != "Food" {
		t.Errorf("row0 = %+v", rows[0])
	}
	// merchant maps to Description; string amount passes through.
	if rows[1].Description != "Payday" || rows[1].Amount != "2500.00" {
		t.Errorf("row1 = %+v", rows[1])
	}
}

func TestParseRowsObjectWrapper(t *testing.T) {
	in := `{"transactions":[{"description":"Rent","amount":-1200}]}`
	rows, err := ParseRows(in)
	if err != nil {
		t.Fatalf("ParseRows: %v", err)
	}
	if len(rows) != 1 || rows[0].Description != "Rent" || rows[0].Amount != "-1200" {
		t.Errorf("rows = %+v", rows)
	}
}

func TestParseRowsCodeFence(t *testing.T) {
	in := "```json\n[{\"description\":\"Tea\",\"amount\":3}]\n```"
	rows, err := ParseRows(in)
	if err != nil {
		t.Fatalf("ParseRows: %v", err)
	}
	if len(rows) != 1 || rows[0].Description != "Tea" || rows[0].Amount != "3" {
		t.Errorf("rows = %+v", rows)
	}
}

func TestParseRowsSkipsEmpty(t *testing.T) {
	in := `[{"note":"ignored"},{"description":"Bus","amount":2}]`
	rows, err := ParseRows(in)
	if err != nil {
		t.Fatalf("ParseRows: %v", err)
	}
	if len(rows) != 1 || rows[0].Description != "Bus" {
		t.Errorf("expected only Bus, got %+v", rows)
	}
}

func TestParseRowsKeepsSourceAccountCurrencyAndStatus(t *testing.T) {
	rows, err := ParseRows(`[{"date":"2026-06-01","description":"Coffee","amount":-4.5,"account_id":"checking","currency":"usd","cleared":true},{"date":"2026-06-01","description":"Coffee","amount":-4.5,"pending":true}]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].AccountID != "checking" || rows[0].Currency != "USD" || rows[0].Status != "cleared" || rows[1].Status != "pending" {
		t.Fatalf("parsed rows = %+v", rows)
	}
}

func TestSignatureNormalizesAmount(t *testing.T) {
	a := Row{Date: "2026-06-01", Amount: "-4.5"}
	b := Row{Date: "2026-06-01", Amount: "-4.50"}
	c := Row{Date: "2026-06-01", Amount: "$4.50"}
	if a.Signature() != b.Signature() {
		t.Errorf("-4.5 and -4.50 should match: %q vs %q", a.Signature(), b.Signature())
	}
	if a.Signature() == c.Signature() {
		t.Error("-4.50 and 4.50 (different sign) should not match")
	}
	// Merchant text is part of the signature so distinct same-day purchases survive.
	d := Row{Date: "2026-06-01", Amount: "-4.5", Description: "Coffee"}
	if a.Signature() == d.Signature() {
		t.Error("description should affect the signature")
	}
	e := Row{Date: "2026-06-01", Amount: "-4.50", Description: "  COFFEE  "}
	if d.Signature() != e.Signature() {
		t.Error("merchant whitespace and case should normalize")
	}
}

func TestParseRowsBadJSON(t *testing.T) {
	if _, err := ParseRows("not json at all"); err == nil {
		t.Error("expected an error for non-JSON")
	}
	if _, err := ParseRows(`{"foo":1}`); err == nil {
		t.Error("expected an error when no list key is present")
	}
}
