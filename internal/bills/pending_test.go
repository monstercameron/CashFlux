// SPDX-License-Identifier: MIT

package bills

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/billmatch"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestPendingInWindow(t *testing.T) {
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	bill := func(name string, minor int64, due time.Time) Bill {
		acct := ""
		if name == "Car Loan" {
			acct = "acct-car"
		}
		return Bill{AccountID: acct, Name: name, Amount: money.New(minor, "USD"), DueDate: due}
	}
	carDue := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	hoaDue := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	augDue := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	pastDue := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)

	all := []Bill{
		bill("Car Loan", 48000, carDue),
		bill("HOA dues", 38000, hoaDue),
		bill("Mortgage", 148000, augDue),  // next month — outside the window
		bill("Water bill", 6000, pastDue), // already behind now — not "upcoming"
	}

	tests := []struct {
		name    string
		txns    []billmatch.Txn
		settled map[string]bool
		want    []string
	}{
		{"nothing posted → both this-month bills pending", nil, nil, []string{"Car Loan", "HOA dues"}},
		{"car paid early → suppressed", []billmatch.Txn{
			{ID: "t1", Date: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC), Payee: "Car Loan", AmountMinor: -48000, Currency: "USD"},
		}, nil, []string{"HOA dues"}},
		{"unrelated posting → still pending", []billmatch.Txn{
			{ID: "t2", Date: carDue, Payee: "Groceries", AmountMinor: -4800, Currency: "USD"},
		}, nil, []string{"Car Loan", "HOA dues"}},
		{"explicitly linked payment settles by account id", nil,
			map[string]bool{"acct-car": true}, []string{"HOA dues"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := PendingInWindow(all, tc.txns, tc.settled, now, end)
			names := make([]string, 0, len(got))
			for _, b := range got {
				names = append(names, b.Name)
			}
			if len(names) != len(tc.want) {
				t.Fatalf("PendingInWindow = %v, want %v", names, tc.want)
			}
			for i := range names {
				if names[i] != tc.want[i] {
					t.Fatalf("PendingInWindow = %v, want %v", names, tc.want)
				}
			}
		})
	}
}
