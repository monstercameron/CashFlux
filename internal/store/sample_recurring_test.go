// SPDX-License-Identifier: MIT

package store

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
)

func TestOctoberSampleCarPaymentsAreRecurringTransfers(t *testing.T) {
	ds := SampleDatasetAt(time.Date(2026, time.November, 19, 0, 0, 0, 0, time.UTC))
	want := map[string]string{
		"Car payment (Priya)":  "acct-carloan-priya",
		"Car payment (Marcus)": "acct-carloan-marcus",
	}
	found := map[string]int{}
	for _, txn := range ds.Transactions {
		destination, ok := want[txn.Desc]
		if !ok || txn.Date.Year() != 2026 || txn.Date.Month() != time.October {
			continue
		}
		if txn.TransferAccountID == "" || txn.CategoryID != "" || txn.Source != domain.TxnSourceRecurring {
			t.Errorf("sample car payment is not a recurring transfer: %#v", txn)
			continue
		}
		if txn.AccountID != destination && txn.TransferAccountID != destination {
			t.Errorf("sample car payment %q does not point to %q: %#v", txn.Desc, destination, txn)
		}
		found[txn.Desc]++
	}
	for label, count := range found {
		if count != 2 {
			t.Errorf("%s has %d transfer legs, want 2", label, count)
		}
	}
	for label := range want {
		if found[label] != 2 {
			t.Errorf("%s found %d transfer legs, want 2", label, found[label])
		}
	}
}

func TestRecurringPaymentCheckFlagsSampleExpenseRows(t *testing.T) {
	ds := SampleDatasetAt(time.Date(2026, time.November, 19, 0, 0, 0, 0, time.UTC))
	for i := range ds.Transactions {
		txn := &ds.Transactions[i]
		if txn.Date.Year() != 2026 || txn.Date.Month() != time.October || !txn.Amount.IsNegative() {
			continue
		}
		if txn.Desc == "Car payment (Priya)" || txn.Desc == "Car payment (Marcus)" {
			// Model the two legacy rows identified by the incident while keeping the
			// real sample itself on the correct transfer representation.
			txn.TransferAccountID = ""
			txn.CategoryID = "cat-autoloan"
		}
	}
	suggestions := txnclassify.FindRecurringPaymentSuggestions(ds.Transactions, ds.Accounts, ds.Recurring)
	if len(suggestions) != 2 {
		t.Fatalf("sample legacy expense suggestions = %#v, want Priya and Marcus payments", suggestions)
	}
	accounts := map[string]string{}
	for _, suggestion := range suggestions {
		accounts[suggestion.AccountName] = suggestion.TransactionID
	}
	if accounts["Priya's Car Loan"] == "" || accounts["Marcus's Car Loan"] == "" {
		t.Fatalf("sample suggestions resolved wrong loan accounts: %#v", suggestions)
	}
}
