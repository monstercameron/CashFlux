// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

func TestTXB4DescriptionSortUsesDisplayedDescription(t *testing.T) {
	input := []domain.Transaction{
		{ID: "raw-first", Payee: "store-a", Desc: "A raw import"},
		{ID: "alias-first", Payee: "store-b", Desc: "Z raw import"},
		{ID: "blank", Payee: "store-c", Desc: ""},
		{ID: "tie-b", Payee: "", Desc: "same"},
		{ID: "tie-a", Payee: "", Desc: "Same"},
	}
	resolve := func(raw string) string {
		return map[string]string{"store-a": "Zulu Market", "store-b": "Alpha Market", "store-c": "Middle Market"}[raw]
	}
	hasLearned := func(raw string) bool { return raw == "store-a" || raw == "store-b" }
	for _, tc := range []struct {
		name string
		dir  string
		want []string
	}{
		{name: "ascending", dir: txnfilter.Asc, want: []string{"alias-first", "blank", "tie-a", "tie-b", "raw-first"}},
		{name: "descending", dir: txnfilter.Desc, want: []string{"raw-first", "tie-a", "tie-b", "blank", "alias-first"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transactions := append([]domain.Transaction(nil), input...)
			sortTxnDescriptions(transactions, tc.dir, hasLearned, resolve)
			for i, id := range tc.want {
				if transactions[i].ID != id {
					t.Fatalf("row %d = %q, want %q in displayed-description order", i, transactions[i].ID, id)
				}
			}
		})
	}
}

func TestTXB4RegisterModeHasNoSortableHeaderKeys(t *testing.T) {
	for _, key := range []string{"date", "amount", "payee", "account", "category", "source"} {
		if got := txnTableSortKey(true, key); got != "" {
			t.Errorf("register-mode %s header sort key = %q, want disabled", key, got)
		}
		if got := txnTableSortKey(false, key); got != key {
			t.Errorf("ordinary-mode %s header sort key = %q, want %q", key, got, key)
		}
	}
}
