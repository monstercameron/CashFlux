// SPDX-License-Identifier: MIT

package txnclassify

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// accounts used by every test: a checking account, a savings account, and a
// credit card, so both the neutral and the debt path have a real counterparty.
func testAccounts() []domain.Account {
	return []domain.Account{
		{ID: "chk", Name: "SCCU Checkings", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"},
		{ID: "sav", Name: "SCCU Savings", Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD"},
		{ID: "card", Name: "Apple Credit Card", Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD"},
	}
}

func spend(amountMinor int64) domain.Transaction {
	return domain.Transaction{
		ID: "t1", AccountID: "chk", Desc: "Transfer to Savings *6500",
		Amount: money.New(amountMinor, "USD"),
		Date:   time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC),
	}
}

func TestKindOf(t *testing.T) {
	accs := testAccounts()
	tests := []struct {
		name string
		txn  domain.Transaction
		want Kind
	}{
		{"plain expense", spend(-50000), KindPlain},
		{"plain income", spend(50000), KindPlain},
		{"transfer to an asset", func() domain.Transaction {
			x := spend(-50000)
			x.TransferAccountID = "sav"
			return x
		}(), KindNeutral},
		{"transfer to a liability", func() domain.Transaction {
			x := spend(-50000)
			x.TransferAccountID = "card"
			return x
		}(), KindDebt},
		{"transfer to a deleted account still is not spending", func() domain.Transaction {
			x := spend(-50000)
			x.TransferAccountID = "gone"
			return x
		}(), KindNeutral},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := KindOf(tt.txn, accs); got != tt.want {
				t.Errorf("KindOf = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestKindOfEmptyAccountList(t *testing.T) {
	x := spend(-50000)
	x.TransferAccountID = "card"
	if got := KindOf(x, nil); got != KindNeutral {
		t.Errorf("with no accounts loaded KindOf = %q, want %q (still not spending)", got, KindNeutral)
	}
}

func TestCounterpartiesExcludesOwnAccount(t *testing.T) {
	got := Counterparties(spend(-50000), testAccounts())
	if len(got) != 2 {
		t.Fatalf("got %d choices, want 2", len(got))
	}
	for _, c := range got {
		if c.AccountID == "chk" {
			t.Errorf("own account offered as a counterparty")
		}
	}
	if got[0].AccountID != "sav" || got[0].IsDebt {
		t.Errorf("choice 0 = %+v, want sav and not a debt", got[0])
	}
	if got[1].AccountID != "card" || !got[1].IsDebt {
		t.Errorf("choice 1 = %+v, want card marked as a debt", got[1])
	}
	if got[1].Name != "Apple Credit Card" {
		t.Errorf("name = %q, want the account's display name", got[1].Name)
	}
}

func TestCounterpartiesPreservesInputOrder(t *testing.T) {
	accs := []domain.Account{
		{ID: "z", Name: "Zulu", Class: domain.ClassAsset},
		{ID: "a", Name: "Alpha", Class: domain.ClassAsset},
		{ID: "chk", Name: "Checking", Class: domain.ClassAsset},
	}
	got := Counterparties(spend(-1), accs)
	if len(got) != 2 || got[0].AccountID != "z" || got[1].AccountID != "a" {
		t.Errorf("order not preserved: %+v", got)
	}
}

func TestCounterpartiesEmptyWhenOnlyOwnAccountExists(t *testing.T) {
	accs := []domain.Account{{ID: "chk", Name: "Checking", Class: domain.ClassAsset}}
	if got := Counterparties(spend(-1), accs); len(got) != 0 {
		t.Errorf("got %d choices, want none", len(got))
	}
}

func TestApplyNeutralTransfer(t *testing.T) {
	in := spend(-50000)
	out, err := Apply(in, "sav", false, testAccounts())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if out.TransferAccountID != "sav" {
		t.Errorf("TransferAccountID = %q, want sav", out.TransferAccountID)
	}
	if out.BillAccountID != "" {
		t.Errorf("BillAccountID = %q, want empty on a neutral move", out.BillAccountID)
	}
	if !out.IsTransfer() || out.IsExpense() || out.IsIncome() {
		t.Errorf("row still reads as income/expense: transfer=%v expense=%v income=%v",
			out.IsTransfer(), out.IsExpense(), out.IsIncome())
	}
	if KindOf(out, testAccounts()) != KindNeutral {
		t.Errorf("KindOf = %q, want neutral", KindOf(out, testAccounts()))
	}
}

func TestApplyDebtPayment(t *testing.T) {
	in := spend(-51848)
	out, err := Apply(in, "card", true, testAccounts())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if out.TransferAccountID != "card" {
		t.Errorf("TransferAccountID = %q, want card", out.TransferAccountID)
	}
	if out.BillAccountID != "card" {
		t.Errorf("BillAccountID = %q, want card so the debt surfaces see the payment", out.BillAccountID)
	}
	if KindOf(out, testAccounts()) != KindDebt {
		t.Errorf("KindOf = %q, want debt", KindOf(out, testAccounts()))
	}
}

func TestApplyDebtCounterpartyWithoutTheBillLink(t *testing.T) {
	// Pointing at a card WITHOUT ticking "count as a payment" still reduces the
	// debt (that is the liability signing in appstate, not this package) and still
	// reads as KindDebt — it just is not claimed as THE monthly payment.
	out, err := Apply(spend(-51848), "card", false, testAccounts())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if out.BillAccountID != "" {
		t.Errorf("BillAccountID = %q, want empty when the box is not ticked", out.BillAccountID)
	}
	if KindOf(out, testAccounts()) != KindDebt {
		t.Errorf("KindOf = %q, want debt from the counterparty's class alone", KindOf(out, testAccounts()))
	}
}

func TestApplyNeverTouchesTheMoney(t *testing.T) {
	for _, amount := range []int64{-50000, 50000, -51848, 1} {
		in := spend(amount)
		in.CategoryID = "cat-transfers"
		in.Tags = []string{"imported"}
		in.Note = "keep me"
		for _, target := range []string{"sav", "card", ""} {
			out, err := Apply(in, target, false, testAccounts())
			if err != nil {
				t.Fatalf("Apply(%q): %v", target, err)
			}
			if out.Amount != in.Amount {
				t.Errorf("amount changed: %v -> %v", in.Amount, out.Amount)
			}
			if !out.Date.Equal(in.Date) {
				t.Errorf("date changed: %v -> %v", in.Date, out.Date)
			}
			if out.CategoryID != in.CategoryID {
				t.Errorf("category changed: %q -> %q", in.CategoryID, out.CategoryID)
			}
			if out.Desc != in.Desc || out.Note != in.Note || out.AccountID != in.AccountID {
				t.Errorf("an unrelated field changed: %+v", out)
			}
		}
	}
}

func TestApplyClearsBackToPlain(t *testing.T) {
	in, err := Apply(spend(-51848), "card", true, testAccounts())
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	out, err := Apply(in, "", false, testAccounts())
	if err != nil {
		t.Fatalf("Apply clear: %v", err)
	}
	if out.TransferAccountID != "" {
		t.Errorf("TransferAccountID = %q, want cleared", out.TransferAccountID)
	}
	if out.BillAccountID != "" {
		t.Errorf("BillAccountID = %q, want cleared with it — the link only means something while the row names a debt", out.BillAccountID)
	}
	if !out.IsExpense() {
		t.Errorf("cleared row should read as spending again")
	}
	if KindOf(out, testAccounts()) != KindPlain {
		t.Errorf("KindOf = %q, want plain", KindOf(out, testAccounts()))
	}
}

func TestApplySwitchingCounterpartyDropsAStaleBillLink(t *testing.T) {
	in, err := Apply(spend(-50000), "card", true, testAccounts())
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	out, err := Apply(in, "sav", false, testAccounts())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if out.BillAccountID != "" {
		t.Errorf("BillAccountID = %q — a payment link to the card survived a move to savings", out.BillAccountID)
	}
}

func TestApplyErrors(t *testing.T) {
	accs := testAccounts()
	tests := []struct {
		name            string
		counterparty    string
		countTowardDebt bool
	}{
		{"own account", "chk", false},
		{"unknown account", "nope", false},
		{"debt payment toward an asset", "sav", true},
		{"debt payment toward nothing", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := spend(-50000)
			out, err := Apply(in, tt.counterparty, tt.countTowardDebt, accs)
			if err == nil {
				t.Fatalf("want an error, got none")
			}
			if out.TransferAccountID != in.TransferAccountID || out.BillAccountID != in.BillAccountID {
				t.Errorf("a rejected Apply modified the transaction: %+v", out)
			}
		})
	}
}

func TestApplyMarksReviewedOnlyWhenSomethingChanged(t *testing.T) {
	accs := testAccounts()

	fresh := spend(-50000)
	if fresh.Reviewed {
		t.Fatalf("fixture should start unreviewed")
	}
	classified, err := Apply(fresh, "sav", false, accs)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !classified.Reviewed {
		t.Errorf("classifying a row is an explicit decision and should mark it reviewed")
	}

	// Re-applying the same classification is not a new decision.
	again := classified
	again.Reviewed = false
	out, err := Apply(again, "sav", false, accs)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if out.Reviewed {
		t.Errorf("re-applying an unchanged classification should not silently mark the row reviewed")
	}

	// Clearing a classification is also a decision.
	cleared, err := Apply(classified, "", false, accs)
	if err != nil {
		t.Fatalf("Apply clear: %v", err)
	}
	if !cleared.Reviewed {
		t.Errorf("clearing a classification should mark the row reviewed")
	}

	// Clearing an already-plain row decides nothing.
	plain := spend(-50000)
	out, err = Apply(plain, "", false, accs)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if out.Reviewed {
		t.Errorf("clearing an already-plain row should not mark it reviewed")
	}
}

func TestApplyIsIdempotent(t *testing.T) {
	accs := testAccounts()
	once, err := Apply(spend(-51848), "card", true, accs)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	twice, err := Apply(once, "card", true, accs)
	if err != nil {
		t.Fatalf("Apply again: %v", err)
	}
	if once.TransferAccountID != twice.TransferAccountID || once.BillAccountID != twice.BillAccountID ||
		once.Amount != twice.Amount {
		t.Errorf("not idempotent:\n once  = %+v\n twice = %+v", once, twice)
	}
}

func TestApplyOnAnIncomeLegKeepsItsSign(t *testing.T) {
	// The misfiled savings leg from a real import: +$500 sitting on checking. It is
	// the FAR side of a move, and its sign says so. Classifying must not flip it.
	in := spend(50000)
	in.Desc = "Transfer from Checking *8945"
	out, err := Apply(in, "sav", false, testAccounts())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if out.Amount.Amount != 50000 {
		t.Errorf("amount = %d, want the incoming leg's sign preserved", out.Amount.Amount)
	}
	if out.IsIncome() {
		t.Errorf("a classified leg must stop reading as income")
	}
}
