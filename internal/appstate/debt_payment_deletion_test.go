// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestTransactionDeletionCleansDebtPaymentLinks(t *testing.T) {
	for _, tc := range []struct {
		name   string
		delete func(*App) error
	}{
		{name: "single", delete: func(a *App) error { return a.DeleteTransaction("payment") }},
		{name: "bulk", delete: func(a *App) error {
			_, err := a.DeleteTransactionsBulk([]string{"payment"})
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newApp(t, false)
			for _, account := range []domain.Account{{ID: "checking"}, {ID: "loan", Class: domain.ClassLiability}} {
				if err := a.Store().PutAccount(account); err != nil {
					t.Fatal(err)
				}
			}
			for _, txn := range []domain.Transaction{{ID: "payment"}, {ID: "paired-row"}, {ID: "unrelated"}} {
				if err := a.Store().PutTransaction(txn); err != nil {
					t.Fatal(err)
				}
			}
			for _, link := range []domain.DebtPaymentLink{
				{ID: "contains-deleted", TransactionIDs: []string{"payment", "paired-row"}, DebtAccountID: "loan"},
				{ID: "keep", TransactionIDs: []string{"unrelated"}, DebtAccountID: "loan"},
			} {
				if err := a.Store().PutDebtPaymentLink(link); err != nil {
					t.Fatal(err)
				}
			}
			if err := tc.delete(a); err != nil {
				t.Fatalf("delete: %v", err)
			}
			links := a.DebtPaymentLinks()
			if len(links) != 1 || links[0].ID != "keep" {
				t.Fatalf("debt-payment links after deletion = %#v", links)
			}
			for _, id := range []string{"payment"} {
				if _, exists, err := a.Store().GetTransaction(id); err != nil || exists {
					t.Fatalf("deleted transaction %q exists=%v err=%v", id, exists, err)
				}
			}
			if _, exists, err := a.Store().GetTransaction("paired-row"); err != nil || !exists {
				t.Fatalf("non-selected transaction was removed: exists=%v err=%v", exists, err)
			}
		})
	}
}

func TestAccountDeletionCleansDebtPaymentReferencesAtomically(t *testing.T) {
	a := newApp(t, false)
	for _, account := range []domain.Account{{ID: "checking"}, {ID: "loan", Class: domain.ClassLiability}, {ID: "other", Class: domain.ClassLiability}} {
		if err := a.Store().PutAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.Store().PutDebtPaymentLink(domain.DebtPaymentLink{ID: "target-link", DebtAccountID: "checking"}); err != nil {
		t.Fatal(err)
	}
	if err := a.Store().PutDebtPaymentLink(domain.DebtPaymentLink{ID: "keep-link", DebtAccountID: "other"}); err != nil {
		t.Fatal(err)
	}
	for _, rule := range []domain.DebtPayeeRule{
		{ID: "target-rule", WorkspaceID: "w", DebtAccountID: "checking"},
		{ID: "source-rule", WorkspaceID: "w", DebtAccountID: "loan", SourceAccountID: "checking"},
		{ID: "keep-rule", WorkspaceID: "w", DebtAccountID: "other", SourceAccountID: "loan"},
	} {
		if err := a.Store().PutDebtPayeeRule(rule); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.DeleteAccount("checking"); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	ds, err := a.Store().Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(ds.DebtPaymentLinks) != 1 || ds.DebtPaymentLinks[0].ID != "keep-link" {
		t.Fatalf("debt-payment links after account deletion = %#v", ds.DebtPaymentLinks)
	}
	if len(ds.DebtPayeeRules) != 1 || ds.DebtPayeeRules[0].ID != "keep-rule" {
		t.Fatalf("debt payee rules after account deletion = %#v", ds.DebtPayeeRules)
	}
	if _, exists, err := a.Store().GetAccount("checking"); err != nil || exists {
		t.Fatalf("deleted account exists=%v err=%v", exists, err)
	}
}
