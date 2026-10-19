// SPDX-License-Identifier: MIT

package store

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestDebtPaymentSQLiteSnapshotRoundTrip(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	link := domain.DebtPaymentLink{ID: "link", TransactionIDs: []string{"t1", "t2"}, DebtAccountID: "loan", Currency: "USD", PrincipalMinor: 8000, InterestMinor: 1000, FeesMinor: 500, EscrowMinor: 250, Method: domain.DebtPaymentStatement, Source: domain.DebtPaymentConfirmed, CreatedRevision: 17}
	rule := domain.DebtPayeeRule{ID: "rule", WorkspaceID: "workspace-a", Payee: "BANK PAYMENT", DebtAccountID: "loan", SourceAccountID: "checking", CreatedAt: time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)}
	if err := s.PutDebtPaymentLink(link); err != nil {
		t.Fatalf("PutDebtPaymentLink: %v", err)
	}
	if err := s.PutDebtPayeeRule(rule); err != nil {
		t.Fatalf("PutDebtPayeeRule: %v", err)
	}
	want := DebtPaymentSnapshot{Links: []domain.DebtPaymentLink{link}, Rules: []domain.DebtPayeeRule{rule}}
	got, err := s.SnapshotDebtPayments()
	if err != nil {
		t.Fatalf("SnapshotDebtPayments: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshot = %#v, want %#v", got, want)
	}
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	var decoded DebtPaymentSnapshot
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	if !reflect.DeepEqual(decoded, want) {
		t.Fatalf("JSON round-trip = %#v, want %#v", decoded, want)
	}
}

func TestDebtPaymentLinkWriteIsAtomic(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	err = s.PutDebtPaymentLink(domain.DebtPaymentLink{ID: "bad", TransactionIDs: []string{"valid", ""}, DebtAccountID: "loan"})
	if err == nil {
		t.Fatal("expected empty transaction id to fail")
	}
	got, err := s.ListDebtPaymentLinks()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("failed write left a link behind: %#v", got)
	}
}

func TestDeletingTransactionsOrAccountsCleansDebtPayments(t *testing.T) {
	for _, deleted := range []string{"transaction", "account"} {
		t.Run(deleted, func(t *testing.T) {
			s, err := NewMemory()
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if err := s.PutAccount(domain.Account{ID: "loan"}); err != nil {
				t.Fatal(err)
			}
			if err := s.PutTransaction(domain.Transaction{ID: "payment", AccountID: "checking"}); err != nil {
				t.Fatal(err)
			}
			if err := s.PutDebtPaymentLink(domain.DebtPaymentLink{ID: "link", TransactionIDs: []string{"payment"}, DebtAccountID: "loan"}); err != nil {
				t.Fatal(err)
			}
			if deleted == "transaction" {
				if _, err := s.DeleteTransactionWithDebtPayments("payment"); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := s.PutDebtPayeeRule(domain.DebtPayeeRule{ID: "rule", WorkspaceID: "w", Payee: "lender", DebtAccountID: "loan"}); err != nil {
					t.Fatal(err)
				}
				if _, err := s.DeleteAccountWithDebtPayments("loan"); err != nil {
					t.Fatal(err)
				}
				rules, err := s.ListDebtPayeeRules()
				if err != nil {
					t.Fatal(err)
				}
				if len(rules) != 0 {
					t.Fatalf("deleting account left payee rule: %#v", rules)
				}
			}
			links, err := s.ListDebtPaymentLinks()
			if err != nil {
				t.Fatal(err)
			}
			if len(links) != 0 {
				t.Fatalf("deleting %s left debt-payment link: %#v", deleted, links)
			}
		})
	}
}
