// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"strconv"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestAutoBudgetOpensWithoutErrorWhenSuggestionsSucceed(t *testing.T) {
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("appstate.New: %v", err)
	}
	if err := app.PutCategory(domain.Category{ID: "groceries", Name: "Groceries", Kind: domain.KindExpense}); err != nil {
		t.Fatalf("PutCategory: %v", err)
	}
	if err := app.PutAccount(domain.Account{ID: "checking", Name: "Checking", Currency: "USD", Type: domain.TypeChecking,
		Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	for month := 1; month <= autoBudgetRecentMonths; month++ {
		txn := domain.Transaction{ID: "grocery-history-" + strconv.Itoa(month), AccountID: "checking", Desc: "Groceries",
			Date: time.Now().AddDate(0, -month, -3), CategoryID: "groceries", Amount: money.New(-12_000, "USD")}
		if err := app.PutTransaction(txn); err != nil {
			t.Fatalf("PutTransaction: %v", err)
		}
	}
	previous := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = previous })
	t.Cleanup(func() { _ = app.Store().Close() })

	// Unmount and drain scheduled field/render callbacks before the app fixture
	// closes the store or restores Default. Cleanup callbacks run in reverse order.
	t.Cleanup(func() { time.Sleep(650 * time.Millisecond) })
	f := render.New(t)
	t.Cleanup(func() {
		f.Render(ui.Fragment())
		f.Stabilize()
		f.Cleanup()
	})
	f.Render(ui.CreateElement(AutoBudgetBody, struct{}{}))
	f.Stabilize()
	if row := byTestID(f, "div", "autobudget-row-groceries"); row == nil {
		t.Fatalf("Auto Budget did not render its spending suggestion: %q", f.Text())
	}
	for _, testID := range []string{
		"autobudget-suggestion-error",
		"autobudget-impact-error",
		"autobudget-income-incomplete",
	} {
		if node := byTestID(f, "p", testID); node != nil {
			t.Errorf("unexpected Auto Budget error banner %s: %q", testID, node.Text())
		}
	}
}
