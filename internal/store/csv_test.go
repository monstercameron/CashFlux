package store

import (
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
