// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestBV6HZeroSpendBudgetStillRendersPaceTick(t *testing.T) {
	f := render.New(t)
	status := budgeting.Status{
		Budget: domain.Budget{ID: "zero", Name: "Groceries", Limit: money.New(10000, "USD")},
		Limit:  money.New(10000, "USD"),
		Spent:  money.New(0, "USD"),
	}
	f.Render(budgetRowCompact(budgetRowCompactProps{
		budgetRowProps: budgetRowProps{PaceMarkerPct: 38},
		Status:         status,
		Width:          0,
	}))
	if tick := byTestID(f, "div", "budget-pace-tick-zero"); tick == nil {
		t.Fatal("current zero-spend budget did not render its pace tick")
	}
}

func TestBV6HRecurringFXExclusionNamesBill(t *testing.T) {
	amount, excluded, err := recurringBudgetAmount(
		domain.Recurring{Label: "Car payment", Amount: money.New(-5000, "EUR"), Cadence: domain.CadenceMonthly},
		"USD", currency.Rates{Base: "USD"},
	)
	if err == nil || amount != 0 || excluded != "Car payment" {
		t.Fatalf("missing FX result = (%d, %q, %v), want named exclusion", amount, excluded, err)
	}
	unnamed := recurringFXExcludedLabel(domain.Recurring{})
	if strings.TrimSpace(unnamed) == "" {
		t.Fatal("unnamed excluded bill should have readable fallback text")
	}
}

func TestBV6HMemberScopedBudgetAddsOthersOnlyToThatCategory(t *testing.T) {
	budgets := []domain.Budget{{Scope: domain.ScopeIndividual, OwnerID: "alice", CategoryID: "food"}}
	categories := []domain.Category{{ID: "food"}}
	if !hasMemberBudgetForCategory(budgets, categories, "food") {
		t.Fatal("category with a member budget should be marked as others-scoped")
	}
	if hasMemberBudgetForCategory(budgets, categories, "home") {
		t.Fatal("category without a member budget should not be marked as others-scoped")
	}
}

func TestBV6HUnbudgetedCategoryUsesListItemSemantics(t *testing.T) {
	f := render.New(t)
	f.Render(ui.CreateElement(unbudgetedChip, unbudgetedChipProps{CatID: "food", CatName: "Food", SpentStr: "$12.00", LimitMajor: "12"}))
	items := f.AllByTag("li")
	if len(items) != 1 || byTestID(f, "button", "budget-this-food") == nil {
		t.Fatalf("unbudgeted chip should be a named list item, got %d list items", len(items))
	}
}

func TestBV6HRecurringAmountsFollowMemberAccountLens(t *testing.T) {
	accounts := []domain.Account{
		{ID: "alice", Scope: domain.ScopeIndividual, OwnerID: "alice"},
		{ID: "bob", Scope: domain.ScopeIndividual, OwnerID: "bob"},
		{ID: "shared", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID},
	}
	for _, tc := range []struct {
		account string
		want    bool
	}{{"alice", false}, {"bob", true}, {"shared", true}, {"", true}} {
		if got := recurringInMemberLens(domain.Recurring{AccountID: tc.account}, "bob", accounts); got != tc.want {
			t.Errorf("account %q in Bob lens = %t, want %t", tc.account, got, tc.want)
		}
	}
}

func TestBV6HAttentionCopyUsesSingularGrammar(t *testing.T) {
	if got := budgetAttentionLabel(1); got != "1 budget needs attention" {
		t.Fatalf("singular attention copy = %q", got)
	}
	if got := budgetAttentionLabel(2); got != "2 budgets need attention" {
		t.Fatalf("plural attention copy = %q", got)
	}
}

func TestBV6HRolloverDisclosureClosesOnBlur(t *testing.T) {
	f := render.New(t)
	f.Render(ui.CreateElement(budgetRolloverDisclosure, budgetRolloverDisclosureProps{
		BudgetID: "groceries", Label: "Rolls over", Text: "Unused money moves forward.",
	}))
	button := byTestID(f, "button", "budget-glyph-rollover-groceries")
	if button == nil {
		t.Fatal("rollover disclosure button did not render")
	}
	button.Click()
	button = byTestID(f, "button", "budget-glyph-rollover-groceries")
	if button.Attr("aria-expanded") != "true" {
		t.Fatal("click did not open rollover explanation")
	}
	button.Dispatch("onblur", render.Event{})
	button = byTestID(f, "button", "budget-glyph-rollover-groceries")
	if button.Attr("aria-expanded") != "false" {
		t.Fatal("leaving rollover button did not close explanation")
	}
}
