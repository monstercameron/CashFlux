// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestDebtPaymentAppstateWritesAndValidates(t *testing.T) {
	a := newApp(t, false)
	if err := a.Store().PutAccount(domain.Account{ID: "checking"}); err != nil {
		t.Fatal(err)
	}
	if err := a.Store().PutAccount(domain.Account{ID: "loan", Class: domain.ClassLiability}); err != nil {
		t.Fatal(err)
	}
	if err := a.Store().PutTransaction(domain.Transaction{ID: "payment", AccountID: "checking"}); err != nil {
		t.Fatal(err)
	}
	if err := a.PutDebtPaymentLink(domain.DebtPaymentLink{ID: "link", TransactionIDs: []string{"payment"}, DebtAccountID: "loan", Currency: "USD", PrincipalMinor: 9000, InterestMinor: 1000, Estimated: true, Method: domain.DebtPaymentAmortized, Source: domain.DebtPaymentConfirmed}); err != nil {
		t.Fatalf("PutDebtPaymentLink: %v", err)
	}
	links := a.DebtPaymentLinks()
	if len(links) != 1 || links[0].CreatedRevision == 0 || !links[0].Estimated {
		t.Fatalf("DebtPaymentLinks() = %#v", links)
	}
	if err := a.PutDebtPayeeRule(domain.DebtPayeeRule{ID: "rule", WorkspaceID: "workspace-1", Payee: "LENDER AUTOPAY", DebtAccountID: "loan", SourceAccountID: "checking"}); err != nil {
		t.Fatalf("PutDebtPayeeRule: %v", err)
	}
	rules := a.DebtPayeeRules()
	if len(rules) != 1 || rules[0].WorkspaceID != "workspace-1" || rules[0].SourceAccountID != "checking" {
		t.Fatalf("DebtPayeeRules() = %#v", rules)
	}
	if err := a.PutDebtPayeeRule(domain.DebtPayeeRule{ID: "unscoped", Payee: "LENDER", DebtAccountID: "loan"}); err == nil {
		t.Fatal("accepted a payee rule without workspace scope")
	}
	if err := a.PutDebtPaymentLink(domain.DebtPaymentLink{ID: "bad-estimate", TransactionIDs: []string{"payment"}, DebtAccountID: "loan", Method: domain.DebtPaymentAmortized, Source: domain.DebtPaymentRule}); err == nil {
		t.Fatal("accepted an amortization estimate without Estimated=true")
	}
}

func TestDebtPaymentDeleteRemovesOnlyRelationship(t *testing.T) {
	a := newApp(t, false)
	if err := a.Store().PutAccount(domain.Account{ID: "loan", Class: domain.ClassLiability}); err != nil {
		t.Fatal(err)
	}
	if err := a.Store().PutTransaction(domain.Transaction{ID: "payment", AccountID: "checking"}); err != nil {
		t.Fatal(err)
	}
	if err := a.PutDebtPaymentLink(domain.DebtPaymentLink{ID: "link", TransactionIDs: []string{"payment"}, DebtAccountID: "loan", Source: domain.DebtPaymentConfirmed, Method: domain.DebtPaymentStatement}); err != nil {
		t.Fatal(err)
	}
	if err := a.DeleteDebtPaymentLink("link"); err != nil {
		t.Fatal(err)
	}
	if len(a.DebtPaymentLinks()) != 0 {
		t.Fatal("debt-payment link still present")
	}
	if _, ok, err := a.Store().GetTransaction("payment"); err != nil || !ok {
		t.Fatalf("transaction missing after relationship deletion; exists=%v err=%v", ok, err)
	}
}
