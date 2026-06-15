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

	got, err := TransactionsFromCSV(data)
	if err != nil {
		t.Fatalf("from csv: %v", err)
	}
	if !reflect.DeepEqual(got, txns) {
		t.Errorf("round trip mismatch:\n got: %+v\nwant: %+v", got, txns)
	}
}

func TestCSVImportGeneratesMissingID(t *testing.T) {
	in := "date,account_id,desc,amount,currency\n2026-06-03,a1,Coffee,-4.50,USD\n"
	got, err := TransactionsFromCSV([]byte(in))
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
	got, err := TransactionsFromCSV([]byte(in))
	if err != nil {
		t.Fatalf("from csv: %v", err)
	}
	if got[0].ID != "tx9" || got[0].AccountID != "a1" || got[0].Amount.Amount != -450 {
		t.Errorf("parsed wrong: %+v", got[0])
	}
}

func TestCSVImportErrors(t *testing.T) {
	cases := []string{
		"date,account_id,amount\n2026-06-03,a1,-4.50\n",       // missing currency column
		"amount,currency\nnotnumber,USD\n",                    // bad amount
		"amount,currency,date\n-4.50,USD,nope\n",              // bad date
	}
	for i, in := range cases {
		if _, err := TransactionsFromCSV([]byte(in)); err == nil {
			t.Errorf("case %d: expected error", i)
		}
	}
}
