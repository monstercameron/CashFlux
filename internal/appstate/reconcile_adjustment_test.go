// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/reconcile"
)

func TestPostReconcileAdjustmentWritesInTheAccountsConvention(t *testing.T) {
	tests := []struct {
		name       string
		class      domain.AccountClass
		typ        domain.AccountType
		sign       domain.LiabilitySign
		diff       int64
		conv       reconcile.Convention
		wantAmount int64
	}{
		{"asset keeps the stated sign", domain.ClassAsset, domain.TypeChecking, "", 4050, reconcile.AsStated, 4050},
		{"negative-owed debt keeps the stated sign", domain.ClassLiability, domain.TypeCreditCard, domain.LiabilityNegativeOwed, -2500, reconcile.NegativeOwed, -2500},
		{"positive-owed debt flips the stated sign", domain.ClassLiability, domain.TypeCreditCard, domain.LiabilityPositiveOwed, -2500, reconcile.PositiveOwed, 2500},
		{"a wrong convention from the caller is ignored", domain.ClassLiability, domain.TypeCreditCard, domain.LiabilityPositiveOwed, -2500, reconcile.AsStated, 2500},
		{"a zero-opening positive-owed card ignores an inferred negative convention", domain.ClassLiability, domain.TypeCreditCard, domain.LiabilityPositiveOwed, 700, reconcile.NegativeOwed, -700},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newApp(t, false)
			acc := domain.Account{ID: "acc", Name: "Acc", Currency: "USD", Type: tt.typ, Class: tt.class, LiabilitySign: tt.sign,
				OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}
			if err := app.PutAccount(acc); err != nil {
				t.Fatal(err)
			}
			posted, err := app.PostReconcileAdjustment("acc", tt.diff, tt.conv, "Adjustment")
			if err != nil {
				t.Fatal(err)
			}
			if posted.Amount.Amount != tt.wantAmount || posted.Amount.Currency != "USD" {
				t.Fatalf("amount = %v, want %d USD", posted.Amount, tt.wantAmount)
			}
			if !posted.Cleared || !posted.Reviewed || posted.Source != domain.TxnSourceManual || posted.Desc != "Adjustment" {
				t.Fatalf("posted row = %+v, want a cleared, reviewed, manual row described Adjustment", posted)
			}
			got, ok, err := app.Store().GetTransaction(posted.ID)
			if err != nil || !ok {
				t.Fatalf("posted row not stored: ok=%v err=%v", ok, err)
			}
			if got.Amount.Amount != tt.wantAmount {
				t.Fatalf("stored amount = %d, want %d", got.Amount.Amount, tt.wantAmount)
			}
		})
	}
}

func TestPostReconcileAdjustmentRejectsAnUnknownAccount(t *testing.T) {
	app := newApp(t, false)
	if _, err := app.PostReconcileAdjustment("missing", 100, reconcile.AsStated, "x"); err == nil {
		t.Fatal("an adjustment for a missing account was posted")
	}
}

func TestPostReconcileAdjustmentOnceRefusesAMismatchedRetry(t *testing.T) {
	app := newApp(t, false)
	if err := app.PutAccount(refsAccount("acc")); err != nil {
		t.Fatal(err)
	}
	first, err := app.PostReconcileAdjustmentOnce("acc", "key", 500, reconcile.AsStated, "Adjustment")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.PostReconcileAdjustmentOnce("acc", "key", 600, reconcile.AsStated, "Adjustment"); err == nil {
		t.Fatal("a retry with another difference returned the old row silently")
	}
	if _, err := app.PostReconcileAdjustmentOnce("acc", "key", 500, reconcile.AsStated, "Other text"); err == nil {
		t.Fatal("a retry with another description returned the old row silently")
	}
	again, err := app.PostReconcileAdjustmentOnce("acc", "key", 500, reconcile.AsStated, "Adjustment")
	if err != nil || again.ID != first.ID {
		t.Fatalf("matching retry = %+v, %v; want the first row", again, err)
	}
}

func TestPostReconcileAdjustmentKeyIsLengthPrefixed(t *testing.T) {
	app := newApp(t, false)
	for _, id := range []string{"a", "a|b"} {
		if err := app.PutAccount(refsAccount(id)); err != nil {
			t.Fatal(err)
		}
	}
	first, err := app.PostReconcileAdjustmentOnce("a", "b|k", 100, reconcile.AsStated, "x")
	if err != nil {
		t.Fatal(err)
	}
	second, err := app.PostReconcileAdjustmentOnce("a|b", "k", 100, reconcile.AsStated, "x")
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID {
		t.Fatal("two different (account, key) pairs hashed to the same row id")
	}
}
