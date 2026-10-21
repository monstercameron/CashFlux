// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestBudgetBV7ILimitButtonNameIncludesVisibleAmountAndBudget(t *testing.T) {
	guardTestApp(t)
	budget := quickFillFixtureBudget()
	budget.Name = "Groceries"
	limit := money.New(20520, "USD")

	fixture := render.New(t)
	fixture.Render(ui.CreateElement(BudgetRow, budgetRowProps{
		Status: budgeting.Status{
			Budget: budget, Limit: limit, Spent: money.New(10000, "USD"),
			Remaining: money.New(10520, "USD"), Percent: 48, State: budgeting.StateOK,
		},
		Compact: true, OnDelete: func(string) {},
	}))

	button := byTestID(fixture, "button", "budget-limit-btn-"+budget.ID)
	if button == nil {
		t.Fatal("budget row has no limit button")
	}
	if got, want := button.Attr("aria-label"), "$205.20, change Groceries limit"; got != want {
		t.Fatalf("limit button accessible name = %q, want %q", got, want)
	}
}
