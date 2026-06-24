// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/rules"
)

func receiptTestApp(t *testing.T) *App {
	t.Helper()
	a := newApp(t, false)
	if err := a.PutAccount(domain.Account{ID: "a1", Name: "Card", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	for _, c := range []domain.Category{
		{ID: "groc", Name: "Groceries", Kind: domain.KindExpense},
		{ID: "house", Name: "Household", Kind: domain.KindExpense},
	} {
		if err := a.PutCategory(c); err != nil {
			t.Fatalf("PutCategory: %v", err)
		}
	}
	return a
}

func TestImportReceiptSplitsAndMapsCategories(t *testing.T) {
	a := receiptTestApp(t)
	r := extract.Receipt{
		Merchant: "Costco", Total: "15.00",
		Lines: []extract.ReceiptLine{
			{Description: "Milk", Category: "Groceries", Amount: "10.00"},
			{Description: "Paper towels", Category: "Household", Amount: "5.00"},
		},
	}
	tx, err := a.ImportReceipt(r, "a1", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ImportReceipt: %v", err)
	}

	all := a.Transactions()
	if len(all) != 1 {
		t.Fatalf("want exactly one transaction (not N), got %d", len(all))
	}
	if tx.Amount.Amount != -1500 {
		t.Errorf("transaction amount = %d, want -1500 (an expense)", tx.Amount.Amount)
	}
	if len(tx.Splits) != 2 || !tx.SplitsReconcile() {
		t.Fatalf("want 2 reconciling splits, got %+v", tx.Splits)
	}
	if tx.Splits[0].CategoryID != "groc" {
		t.Errorf("milk line mapped to %q, want groc (by name)", tx.Splits[0].CategoryID)
	}
	if tx.Splits[1].CategoryID != "house" {
		t.Errorf("paper towels mapped to %q, want house (by name)", tx.Splits[1].CategoryID)
	}
}

func TestImportReceiptFallsBackToMerchantRule(t *testing.T) {
	a := receiptTestApp(t)
	// A user rule on the merchant; the line has no usable extracted category.
	if err := a.PutRule(rules.Rule{ID: "r1", Match: "Costco", SetCategoryID: "groc"}); err != nil {
		t.Fatalf("PutRule: %v", err)
	}
	r := extract.Receipt{
		Merchant: "Costco", Total: "4.00",
		Lines: []extract.ReceiptLine{{Description: "Item 1", Category: "", Amount: "4.00"}},
	}
	tx, err := a.ImportReceipt(r, "a1", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ImportReceipt: %v", err)
	}
	if tx.Splits[0].CategoryID != "groc" {
		t.Errorf("uncategorized line at Costco mapped to %q, want groc (merchant rule)", tx.Splits[0].CategoryID)
	}
	// A single-category receipt also tags the transaction itself.
	if tx.CategoryID != "groc" {
		t.Errorf("single-category receipt should set tx.CategoryID=groc, got %q", tx.CategoryID)
	}
}

func TestImportReceiptRejectsBadInput(t *testing.T) {
	a := receiptTestApp(t)
	nonReconciling := extract.Receipt{
		Merchant: "Costco", Total: "20.00",
		Lines: []extract.ReceiptLine{{Description: "Milk", Category: "Groceries", Amount: "10.00"}},
	}
	if _, err := a.ImportReceipt(nonReconciling, "a1", time.Now()); err == nil {
		t.Error("expected an error when splits do not sum to the total")
	}
	ok := extract.Receipt{Merchant: "Costco", Total: "10.00", Lines: []extract.ReceiptLine{{Description: "Milk", Category: "Groceries", Amount: "10.00"}}}
	if _, err := a.ImportReceipt(ok, "", time.Now()); err == nil {
		t.Error("expected an error when no account is given")
	}
}
