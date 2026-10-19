// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reviewscope"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

type tcw4LoanProps struct {
	App      *appstate.App
	Revision uint64
}

func tcw4LoanPayments(props tcw4LoanProps) ui.Node {
	c := reviewController{app: props.App}
	return c.useLoanPayments()
}

func TestTCW4ReviewDismissalSurvivesAmountEditAndRemount(t *testing.T) {
	app := newR3EEditApp(t)
	if err := app.PutAccount(domain.Account{ID: "loan", Name: "Priya Car Loan", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassLiability, Type: domain.TypeLoan, Currency: "USD", MinPayment: money.New(48000, "USD")}); err != nil {
		t.Fatal(err)
	}
	recurring := domain.Recurring{ID: "tcw4-recurring", Label: "Car payment Priya", Amount: money.New(-48000, "USD"),
		Cadence: domain.CadenceMonthly, NextDue: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}
	if err := app.PutRecurring(recurring); err != nil {
		t.Fatal(err)
	}
	row := domain.Transaction{ID: "tcw4-payment", AccountID: "acct-1", Payee: recurring.Label, Desc: recurring.Label,
		Amount: recurring.Amount, Date: time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)}
	if err := app.PutTransaction(row); err != nil {
		t.Fatal(err)
	}
	f := render.New(t)
	f.Render(ui.CreateElement(tcw4LoanPayments, tcw4LoanProps{app, app.Rev()}))
	f.Stabilize()
	button := findByTestID(f, "button", "review-loan-payment-dismiss")
	if button == nil {
		t.Fatal("missing initial loan offer")
	}
	button.Click()
	f.Stabilize()
	if !loadRecurringLoanPaymentDismissals()[row.ID] {
		t.Fatal("dismissal was not stored")
	}
	recurring.Amount = money.New(-49000, "USD")
	if err := app.PutRecurring(recurring); err != nil {
		t.Fatal(err)
	}
	if len(app.RecurringPaymentSuggestions()) != 0 {
		t.Fatal("amount edit did not remove the offer")
	}
	f.Rerender(ui.CreateElement(tcw4LoanPayments, tcw4LoanProps{app, app.Rev()}))
	f.Stabilize()
	if !loadRecurringLoanPaymentDismissals()[row.ID] {
		t.Fatal("amount mismatch erased stored dismissal")
	}
	f.Cleanup()
	recurring.Amount = row.Amount
	if err := app.PutRecurring(recurring); err != nil {
		t.Fatal(err)
	}
	f = render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(tcw4LoanPayments, tcw4LoanProps{app, app.Rev()}))
	f.Stabilize()
	if findByTestID(f, "div", "review-loan-payment-group") != nil {
		t.Fatal("dismissed offer returned after reverting the amount and reopening")
	}
	row.TransferAccountID = "loan"
	if err := app.PutTransaction(row); err != nil {
		t.Fatal(err)
	}
	f.Rerender(ui.CreateElement(tcw4LoanPayments, tcw4LoanProps{app, app.Rev()}))
	f.Stabilize()
	if loadRecurringLoanPaymentDismissals()[row.ID] {
		t.Fatal("transfer conversion retained an obsolete dismissal")
	}
}

func TestTCW4ReviewSingleUsesWholeLedgerDuplicateIndex(t *testing.T) {
	app := newReviewApp(t)
	first := putCharge(t, app, "tcw4-dupe-a", "Market", 12)
	second := first
	second.ID = "tcw4-dupe-b"
	second.CategoryID = "cat-food" // Already filed, so outside the open queue.
	if err := app.PutTransaction(second); err != nil {
		t.Fatal(err)
	}
	idx := buildReviewIndex(app, reviewscope.ScopeEntireQueue, txnfilter.Criteria{})
	if len(idx.Dupes[first.ID]) != 2 || idx.TxnByID[second.ID].ID != second.ID {
		t.Fatalf("shared duplicate context did not include the filed charge: %+v", idx.Dupes)
	}
	// The single-charge context consumes this index, including charges outside
	// the review queue; it must not derive duplicates from its one-row scope.
	a := reviewContextBandProps{App: app, Index: idx, Row: idx.Rows[0], Txn: first}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(reviewContextBand, a))
	if findByTestID(f, "div", "review-dupe") == nil {
		t.Fatalf("single-charge review omitted shared duplicates: %s", f.Text())
	}
	a.Index.Dupes = nil
	f.Rerender(ui.CreateElement(reviewContextBand, a))
	if findByTestID(f, "div", "review-dupe") != nil {
		t.Fatal("single-charge view rescanned the ledger instead of consuming its index")
	}
}
