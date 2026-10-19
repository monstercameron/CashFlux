// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reviewscope"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

func TestReviewTC3B6SuggestionOnlyEmptyStateCopy(t *testing.T) {
	title, sub := reviewEmptyCopy(reviewscope.ScopeEntireQueue, 0, 1)
	if title != "A few payments may be loans" || sub != "Choose whether to record each one as a transfer." {
		t.Fatalf("suggestion-only copy = (%q, %q)", title, sub)
	}
}

func TestReviewTC3B6CurrentViewEmptyStateStillNamesQueuedRows(t *testing.T) {
	title, _ := reviewEmptyCopy(reviewscope.ScopeCurrentView, 3, 1)
	if title != uistate.T("review.emptyViewTitle") {
		t.Fatalf("current-view empty title = %q, want the outside-view queue notice", title)
	}
}

func TestReviewTC3B6LoanPaymentMemoUsesAppAndRevision(t *testing.T) {
	var memo recurringLoanPaymentSuggestionMemo
	calls := 0
	load := func() []txnclassify.PaymentSuggestion {
		calls++
		return []txnclassify.PaymentSuggestion{{TransactionID: "payment"}}
	}
	app := &appstate.App{}
	memo.get(app, 4, load)
	memo.get(app, 4, load)
	if calls != 1 {
		t.Fatalf("load calls at same revision = %d, want 1", calls)
	}
	memo.get(app, 5, load)
	if calls != 2 {
		t.Fatalf("load calls after revision change = %d, want 2", calls)
	}
	memo.get(&appstate.App{}, 5, load)
	if calls != 3 {
		t.Fatalf("load calls after app change = %d, want 3", calls)
	}
}

func TestReviewTC3B6DismissalPruningAndVisibility(t *testing.T) {
	dismissed := map[string]bool{"live": true, "gone": true, "false": false}
	suggestions := []txnclassify.PaymentSuggestion{{TransactionID: "live"}, {TransactionID: "new"}}
	pruned, changed := pruneRecurringLoanPaymentDismissals(dismissed, suggestions)
	if !changed || len(pruned) != 1 || !pruned["live"] {
		t.Fatalf("pruned dismissals = %#v, changed=%v", pruned, changed)
	}
	visible := visibleRecurringLoanPaymentSuggestions(suggestions, pruned)
	if len(visible) != 1 || visible[0].TransactionID != "new" {
		t.Fatalf("visible suggestions = %#v, want only new", visible)
	}
}

func TestReviewTC3B6LoanPaymentUndoWaitsForDurableSave(t *testing.T) {
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, account := range []domain.Account{
		{ID: "checking", Name: "Checking", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"},
		{ID: "loan", Name: "Priya Car Loan", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassLiability, Type: domain.TypeLoan, Currency: "USD", MinPayment: money.New(48000, "USD")},
	} {
		if err := app.PutAccount(account); err != nil {
			t.Fatalf("PutAccount: %v", err)
		}
	}
	if err := app.PutRecurring(domain.Recurring{ID: "car-payment", Label: "Car payment Priya", Amount: money.New(-48000, "USD"), Cadence: domain.CadenceMonthly, NextDue: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("PutRecurring: %v", err)
	}
	if err := app.PutTransaction(domain.Transaction{ID: "legacy-payment", AccountID: "checking", Date: time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC), Payee: "Car payment Priya", Desc: "Car payment Priya", Amount: money.New(-48000, "USD")}); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}
	suggestion := app.RecurringPaymentSuggestions()[0]
	var revision uint64
	var finish func(appstate.DurableResult)
	app.SetDurableWriter(func(got uint64, done func(appstate.DurableResult)) {
		revision, finish = got, done
	})
	called := false
	var saveErr error
	recordRecurringLoanPayment(app, suggestion, func(err error) { called, saveErr = true, err })
	if called || finish == nil {
		t.Fatal("record action acknowledged before the durable writer completed")
	}
	finish(appstate.DurableResult{Revision: revision, Err: errors.New("quota exceeded")})
	if !called || saveErr == nil {
		t.Fatalf("durability failure callback = called %v, err %v", called, saveErr)
	}
}
