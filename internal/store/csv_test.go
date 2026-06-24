// SPDX-License-Identifier: MIT

package store

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestTransactionsCSVRoundTrip(t *testing.T) {
	day, _ := dateutil.ParseDate("2026-06-03")
	day2, _ := dateutil.ParseDate("2026-06-05")

	txns := []domain.Transaction{
		{
			ID: "t1", AccountID: "a1", Date: day, Payee: "Store, Inc.", Desc: "Groceries",
			CategoryID: "food", Amount: money.New(-24055, "USD"), Cleared: true,
			Tags: []string{"weekly", "essentials"}, MemberID: "m1",
		},
		{
			ID: "t2", AccountID: "a1", Date: day2, Desc: "Transfer to savings",
			Amount: money.New(-50000, "USD"), TransferAccountID: "a2",
		},
	}

	data, err := TransactionsToCSV(txns)
	if err != nil {
		t.Fatalf("to csv: %v", err)
	}
	if !strings.HasPrefix(string(data), "id,date,account_id") {
		t.Errorf("missing/wrong header: %q", string(data)[:40])
	}

	got, err := TransactionsFromCSV(data, "")
	if err != nil {
		t.Fatalf("from csv: %v", err)
	}
	if !reflect.DeepEqual(got, txns) {
		t.Errorf("round trip mismatch:\n got: %+v\nwant: %+v", got, txns)
	}
}

func TestCSVImportGeneratesMissingID(t *testing.T) {
	in := "date,account_id,desc,amount,currency\n2026-06-03,a1,Coffee,-4.50,USD\n"
	got, err := TransactionsFromCSV([]byte(in), "")
	if err != nil {
		t.Fatalf("from csv: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d transactions, want 1", len(got))
	}
	if got[0].ID == "" {
		t.Error("expected a generated id")
	}
	if got[0].Amount.Amount != -450 || got[0].Amount.Currency != "USD" {
		t.Errorf("amount = %v, want -450 USD", got[0].Amount)
	}
}

func TestCSVImportColumnOrderTolerant(t *testing.T) {
	// Reordered + extra column.
	in := "amount,currency,account_id,extra,id\n-4.50,USD,a1,ignored,tx9\n"
	got, err := TransactionsFromCSV([]byte(in), "")
	if err != nil {
		t.Fatalf("from csv: %v", err)
	}
	if got[0].ID != "tx9" || got[0].AccountID != "a1" || got[0].Amount.Amount != -450 {
		t.Errorf("parsed wrong: %+v", got[0])
	}
}

func TestCSVImportDefaultsCurrencyAndFriendlyColumns(t *testing.T) {
	// The documented hand-written shape: no currency column, friendly column
	// names (account/category/member, not *_id). Currency defaults to the
	// caller-supplied base; the friendly columns are read into the id fields
	// (appstate resolves any names to ids). (C27)
	in := "date,payee,amount,account,category,member\n2026-06-10,Coffee Bar,-4.50,Checking,Food,Alex\n"
	got, err := TransactionsFromCSV([]byte(in), "USD")
	if err != nil {
		t.Fatalf("from csv: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d rows, want 1", len(got))
	}
	g := got[0]
	if g.Amount.Amount != -450 || g.Amount.Currency != "USD" {
		t.Errorf("amount = %v, want -450 USD (defaulted)", g.Amount)
	}
	if g.Payee != "Coffee Bar" || g.AccountID != "Checking" || g.CategoryID != "Food" || g.MemberID != "Alex" {
		t.Errorf("friendly columns not read: %+v", g)
	}
	// The payee fills the (required) description when no desc column is present,
	// so the documented shape actually imports rather than failing validation.
	if g.Desc != "Coffee Bar" {
		t.Errorf("Desc = %q, want the payee fallback %q", g.Desc, "Coffee Bar")
	}
}

func TestCSVImportDescFallsBackToPayee(t *testing.T) {
	// No desc column (the documented date,payee,amount,account shape): the ledger
	// requires a description, so it falls back to the payee.
	got, err := TransactionsFromCSV([]byte("date,payee,amount,account\n2026-06-05,Rent,-1200.00,Checking\n"), "USD")
	if err != nil {
		t.Fatalf("from csv: %v", err)
	}
	if len(got) != 1 || got[0].Desc != "Rent" {
		t.Errorf("Desc = %q, want payee fallback \"Rent\"", got[0].Desc)
	}
	// An explicit desc column takes precedence over the payee.
	got2, err := TransactionsFromCSV([]byte("date,payee,desc,amount,account\n2026-06-05,Rent,Monthly rent,-1200.00,Checking\n"), "USD")
	if err != nil {
		t.Fatalf("from csv: %v", err)
	}
	if got2[0].Desc != "Monthly rent" {
		t.Errorf("Desc = %q, want explicit \"Monthly rent\"", got2[0].Desc)
	}
	if got2[0].Payee != "Rent" {
		t.Errorf("Payee = %q, want \"Rent\"", got2[0].Payee)
	}
}

func TestCSVImportExportIDColumnsWinOverFriendly(t *testing.T) {
	// When both account_id and account are present, the explicit id wins.
	in := "amount,currency,account_id,account\n-1.00,USD,acc-123,Checking\n"
	got, err := TransactionsFromCSV([]byte(in), "")
	if err != nil {
		t.Fatalf("from csv: %v", err)
	}
	if got[0].AccountID != "acc-123" {
		t.Errorf("account_id should win: got %q", got[0].AccountID)
	}
}

func TestCSVImportErrors(t *testing.T) {
	cases := []string{
		"date,account_id,amount\n2026-06-03,a1,-4.50\n", // no currency column AND no default → error
		"amount,currency\nnotnumber,USD\n",              // bad amount
		"amount,currency,date\n-4.50,USD,nope\n",        // bad date
		"date,account_id\n2026-06-03,a1\n",              // missing amount
	}
	for i, in := range cases {
		if _, err := TransactionsFromCSV([]byte(in), ""); err == nil {
			t.Errorf("case %d: expected error", i)
		}
	}
}

func TestTransactionsFromCSVResilient(t *testing.T) {
	t.Run("all_valid", func(t *testing.T) {
		in := "date,payee,amount,currency\n2026-06-01,Coffee,-4.50,USD\n2026-06-02,Salary,2000.00,USD\n"
		txns, skipped, err := TransactionsFromCSVResilient([]byte(in), "")
		if err != nil {
			t.Fatalf("unexpected structural error: %v", err)
		}
		if len(txns) != 2 {
			t.Errorf("got %d txns, want 2", len(txns))
		}
		if len(skipped) != 0 {
			t.Errorf("got %d skipped, want 0: %+v", len(skipped), skipped)
		}
	})

	t.Run("some_bad_rows", func(t *testing.T) {
		// Row on line 2 = missing amount, line 3 = valid, line 4 = non-numeric amount, lines 5+6 = valid.
		in := strings.Join([]string{
			"date,payee,amount,currency",
			"2026-06-01,Bad1,,USD",           // line 2: missing amount
			"2026-06-02,Good1,-4.50,USD",     // line 3: valid
			"2026-06-03,Bad2,notanumber,USD", // line 4: non-numeric amount
			"2026-06-04,Good2,100.00,USD",    // line 5: valid
			"2026-06-05,Good3,-22.00,USD",    // line 6: valid
		}, "\n") + "\n"

		txns, skipped, err := TransactionsFromCSVResilient([]byte(in), "")
		if err != nil {
			t.Fatalf("unexpected structural error: %v", err)
		}
		if len(txns) != 3 {
			t.Errorf("got %d valid txns, want 3", len(txns))
		}
		if len(skipped) != 2 {
			t.Errorf("got %d skipped, want 2: %+v", len(skipped), skipped)
		}
		// Check line numbers.
		lines := make(map[int]bool, len(skipped))
		for _, s := range skipped {
			lines[s.Line] = true
		}
		if !lines[2] {
			t.Errorf("expected line 2 in skipped; got %+v", skipped)
		}
		if !lines[4] {
			t.Errorf("expected line 4 in skipped; got %+v", skipped)
		}
		// Reasons must be non-empty.
		for _, s := range skipped {
			if s.Reason == "" {
				t.Errorf("skipped row at line %d has empty reason", s.Line)
			}
		}
	})

	t.Run("empty_input", func(t *testing.T) {
		txns, skipped, err := TransactionsFromCSVResilient([]byte(""), "USD")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(txns) != 0 || len(skipped) != 0 {
			t.Errorf("expected empty results, got txns=%d skipped=%d", len(txns), len(skipped))
		}
	})

	t.Run("header_only", func(t *testing.T) {
		txns, skipped, err := TransactionsFromCSVResilient([]byte("date,payee,amount,currency\n"), "USD")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(txns) != 0 || len(skipped) != 0 {
			t.Errorf("expected empty results for header-only, got txns=%d skipped=%d", len(txns), len(skipped))
		}
	})

	t.Run("totally_malformed", func(t *testing.T) {
		// A bare quote triggers a CSV parse error — structural failure, not a skipped row.
		_, _, err := TransactionsFromCSVResilient([]byte("date,amount\n\"unclosed\n"), "USD")
		if err == nil {
			t.Error("expected structural error for malformed CSV")
		}
	})
}

// TestCSVResilientScale verifies that TransactionsFromCSVResilient handles a
// 600-row corpus at scale (L23): all valid rows are imported, scattered malformed
// rows are skipped with reasons, and the function never aborts early. This guards
// against any regression that silently returns a partial result or panics on bulk
// data.
func TestCSVResilientScale(t *testing.T) {
	const totalRows = 600
	// Malformed rows at a spread of positions (1-based data row number).
	malformedAt := map[int]bool{1: true, 50: true, 200: true, 350: true, 599: true}
	wantSkipped := len(malformedAt)
	wantImported := totalRows - wantSkipped

	var sb strings.Builder
	sb.WriteString("date,payee,amount,currency\n")
	for i := 1; i <= totalRows; i++ {
		day := fmt.Sprintf("2024-%02d-%02d", (i%12)+1, (i%28)+1)
		if malformedAt[i] {
			// Missing amount — guaranteed skip by the resilient parser.
			fmt.Fprintf(&sb, "%s,Malformed row %d,,USD\n", day, i)
		} else {
			fmt.Fprintf(&sb, "%s,Payee %d,-%.2f,USD\n", day, i, float64(i)*1.23)
		}
	}

	txns, skipped, err := TransactionsFromCSVResilient([]byte(sb.String()), "")
	if err != nil {
		t.Fatalf("unexpected structural error on 600-row corpus: %v", err)
	}
	if len(txns) != wantImported {
		t.Errorf("imported %d rows, want %d", len(txns), wantImported)
	}
	if len(skipped) != wantSkipped {
		t.Errorf("skipped %d rows, want %d: %+v", len(skipped), wantSkipped, skipped)
	}
	for _, s := range skipped {
		if s.Reason == "" {
			t.Errorf("skipped row at line %d has empty reason", s.Line)
		}
		if s.Line < 2 || s.Line > totalRows+1 {
			t.Errorf("skipped line number %d out of expected range [2, %d]", s.Line, totalRows+1)
		}
	}
	// Every imported transaction must have a non-empty ID (generated when absent)
	// and a valid non-zero amount.
	for _, tx := range txns {
		if tx.ID == "" {
			t.Error("imported transaction missing ID")
		}
		if tx.Amount.Amount == 0 {
			t.Errorf("imported transaction %s has zero amount", tx.ID)
		}
	}
}
