// SPDX-License-Identifier: MIT

package smartengine

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/smart"
)

func TestU1UnusualUsesTypedStableKeyAndPlainEvidence(t *testing.T) {
	in := baseInput()
	in.Transactions = []domain.Transaction{{
		ID: "new-shop", AccountID: "checking", Date: ref.AddDate(0, 0, -1),
		Payee: "New Shop", Amount: usd(-30000),
	}}

	got := u1Unusual(in)
	if len(got) != 1 {
		t.Fatalf("want one first-merchant finding, got %+v", got)
	}
	if got[0].Feature != "SMART-U1" || got[0].Key != "SMART-U1:FirstTimeMerchantLarge:new-shop" {
		t.Fatalf("unexpected feature identity: %+v", got[0])
	}
	if got[0].Title == "" || !strings.Contains(got[0].Detail, "no earlier charge from this merchant") || !strings.Contains(got[0].Detail, "confidence low") {
		t.Fatalf("finding should contain readable evidence: %+v", got[0])
	}
	if got[0].Action == nil || got[0].Action.RelatedID != "new-shop" {
		t.Fatalf("finding should link to its transaction: %+v", got[0].Action)
	}
}

func TestU1ExcludesDebtPaymentPrincipalLegs(t *testing.T) {
	in := baseInput()
	in.Accounts = []domain.Account{
		{ID: "checking", Class: domain.ClassAsset, Currency: "USD"},
		{ID: "loan", Class: domain.ClassLiability, Name: "Auto loan", Currency: "USD"},
	}
	txn := domain.Transaction{
		ID: "loan-payment", AccountID: "checking", Date: ref.AddDate(0, 0, -1),
		Payee: "Auto loan", BillAccountID: "loan", Amount: usd(-30000),
	}
	in.Transactions = []domain.Transaction{txn}
	if got := u1Unusual(in); len(got) != 0 {
		t.Fatalf("debt-payment principal leg must be excluded, got %+v", got)
	}
}

func TestU1IsRegisteredAndRespectsSettings(t *testing.T) {
	if !HasEngine("SMART-U1") || HasEngine("SMART-T6") {
		t.Fatal("Unusual should replace T6 in the engine registry")
	}
	in := baseInput()
	in.Transactions = []domain.Transaction{{
		ID: "new-shop", AccountID: "checking", Date: ref.AddDate(0, 0, -1),
		Payee: "New Shop", Amount: usd(-30000),
	}}
	settings := smart.Settings{}.SetEnabled("SMART-U1", false)
	for _, got := range RunPage(in, settings, smart.PageTransactions) {
		if got.Feature == "SMART-U1" {
			t.Fatalf("disabled Unusual should not run, got %+v", got)
		}
	}
}
