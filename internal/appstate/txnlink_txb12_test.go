// SPDX-License-Identifier: MIT

package appstate

import (
	"errors"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func txb12Txn(id string, amount int64, currency string) domain.Transaction {
	return domain.Transaction{
		ID: id, AccountID: "account", Date: time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC),
		Desc: "Jacket Co", Payee: "Jacket Co", Amount: money.New(amount, currency),
	}
}

func TestPutTxnLinkRefundPairInvariants(t *testing.T) {
	tests := []struct {
		name      string
		seed      []domain.Transaction
		links     []domain.TxnLink
		candidate domain.TxnLink
		wantErr   bool
	}{
		{
			name:      "refund can only be paired once",
			seed:      []domain.Transaction{txb12Txn("original-a", -12000, "USD"), txb12Txn("original-b", -12000, "USD"), txb12Txn("refund", 12000, "USD")},
			links:     []domain.TxnLink{{ID: "first", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"original-a", "refund"}}},
			candidate: domain.TxnLink{Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"original-b", "refund"}},
			wantErr:   true,
		},
		{
			name:      "original must be an expense",
			seed:      []domain.Transaction{txb12Txn("original", 12000, "USD"), txb12Txn("refund", 12000, "USD")},
			candidate: domain.TxnLink{Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"original", "refund"}},
			wantErr:   true,
		},
		{
			name:      "refund must be income",
			seed:      []domain.Transaction{txb12Txn("original", -12000, "USD"), txb12Txn("refund", -12000, "USD")},
			candidate: domain.TxnLink{Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"original", "refund"}},
			wantErr:   true,
		},
		{
			name:      "refund and original must use same currency",
			seed:      []domain.Transaction{txb12Txn("original", -12000, "USD"), txb12Txn("refund", 12000, "EUR")},
			candidate: domain.TxnLink{Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"original", "refund"}, Amount: money.New(12000, "EUR")},
			wantErr:   true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := newApp(t, false)
			for _, txn := range tc.seed {
				if err := a.PutTransaction(txn); err != nil {
					t.Fatalf("seed transaction %s: %v", txn.ID, err)
				}
			}
			for _, link := range tc.links {
				if err := a.PutTxnLink(link); err != nil {
					t.Fatalf("seed link: %v", err)
				}
			}
			err := a.PutTxnLink(tc.candidate)
			if (err != nil) != tc.wantErr {
				t.Fatalf("PutTxnLink error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestTxnLinkWritesRespectActiveRole(t *testing.T) {
	for _, tc := range []struct {
		role    domain.MemberRole
		wantErr bool
	}{{domain.RoleOwner, false}, {domain.RoleAdmin, false}, {domain.RoleViewer, true}} {
		t.Run(string(tc.role), func(t *testing.T) {
			a := newApp(t, false)
			a.SetActiveRoleFunc(func() domain.MemberRole { return tc.role })
			putErr := a.PutTxnLink(domain.TxnLink{Kind: domain.TxnLinkOrderGroup, TxnIDs: []string{"a", "b"}})
			if (putErr != nil) != tc.wantErr || (tc.wantErr && !errors.Is(putErr, ErrReadOnly)) {
				t.Errorf("PutTxnLink error = %v, wantErr %v", putErr, tc.wantErr)
			}
			if !tc.wantErr && putErr == nil {
				linkID := a.TxnLinks()[0].ID
				deleteErr := a.DeleteTxnLink(linkID)
				if deleteErr != nil {
					t.Errorf("DeleteTxnLink error = %v, want nil", deleteErr)
				}
				return
			}
			if !tc.wantErr {
				return
			}
			if err := a.store.PutTxnLink(domain.TxnLink{ID: "existing", Kind: domain.TxnLinkOrderGroup, TxnIDs: []string{"a", "b"}}); err != nil {
				t.Fatalf("seed link: %v", err)
			}
			if err := a.DeleteTxnLink("existing"); !errors.Is(err, ErrReadOnly) {
				t.Errorf("DeleteTxnLink error = %v, want ErrReadOnly", err)
			}
		})
	}
}
