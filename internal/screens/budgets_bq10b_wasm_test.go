// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goalfunded"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

func TestBQ10BBudgetSummaryPlanCountsSavingAssignmentOnce(t *testing.T) {
	statuses := []budgeting.Status{
		{Budget: domain.Budget{ID: "spending", Period: domain.PeriodMonthly, Limit: money.New(300_000, "USD")}, Limit: money.New(300_000, "USD")},
		{Budget: domain.Budget{ID: "saving", Period: domain.PeriodMonthly, Direction: domain.DirectionSave, Limit: money.New(50_000, "USD")}, Limit: money.New(50_000, "USD")},
	}
	spending := budgetSummarySpendingBudgets(statuses)
	if len(spending) != 1 || spending[0].ID != "spending" {
		t.Fatalf("normalized plan budgets = %+v, want only spending budget", spending)
	}
	normalized, err := budgeting.NormalizeBudgets(spending, domain.PeriodMonthly, currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	got := budgeting.ToAssign(500_000, normalized.Total.Amount+50_000)
	if got != 150_000 {
		t.Fatalf("To Assign = %d, want 150000 ($1,500) from $5,000 income, $3,000 spending and $500 savings", got)
	}
}

func TestBQ10BBudgetSearchMatchesNameCategoryAndTrackedTag(t *testing.T) {
	b := domain.Budget{Name: "Date nights", CategoryID: "dining", TrackedTags: []string{"Weekend"}}
	categories := map[string]string{"dining": "Dining"}
	for _, query := range []string{"date", "dining", "weekend"} {
		if !budgetSearchMatches(b, categories, query) {
			t.Errorf("budget search %q did not match name, category, or tag", query)
		}
	}
	if budgetSearchMatches(b, categories, "utilities") {
		t.Error("unrelated budget search matched")
	}
}

func TestBQ10BSavingsMonthlyBaseDoesNotFallBackToRawForeignMinorUnits(t *testing.T) {
	if got := savingsMonthlyBase(50_000, "EUR", "USD", currency.Rates{Base: "USD"}); got != 0 {
		t.Fatalf("unconvertible monthly assignment = %d, want zero", got)
	}
	if got := savingsMonthlyBase(50_000, "EUR", "USD", currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.1}}); got != 55_000 {
		t.Fatalf("converted monthly assignment = %d, want 55000 USD minor units", got)
	}
}

func TestBQ10BBudgetCompositionUsesSignedBudgetCurrencySlices(t *testing.T) {
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	b := domain.Budget{ID: "food", CategoryID: "food", Period: domain.PeriodMonthly, Limit: money.New(20_000, "EUR")}
	txn := domain.Transaction{
		ID: "receipt", Date: start.AddDate(0, 0, 3), Amount: money.New(-7_000, "EUR"),
		Splits: []domain.CategorySplit{
			{CategoryID: "groceries", Amount: money.New(-10_000, "EUR")},
			{CategoryID: "discount", Amount: money.New(3_000, "EUR")},
		},
	}
	categories := []domain.Category{
		{ID: "food", Name: "Food"}, {ID: "groceries", Name: "Groceries", ParentID: "food"}, {ID: "discount", Name: "Discount", ParentID: "food"},
	}
	slices, err := budgetCompositionData(b, []domain.Transaction{txn}, categories, start, end, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.1}})
	if err != nil {
		t.Fatal(err)
	}
	var sum int64
	seenNegative := false
	for _, slice := range slices {
		sum += slice.Amount
		if slice.Currency != "EUR" {
			t.Errorf("slice %q currency = %q, want EUR", slice.Label, slice.Currency)
		}
		if slice.Amount < 0 {
			seenNegative = true
			if slice.ValStr == "" || (slice.ValStr[0] != '-' && slice.ValStr[0] != '(') {
				t.Errorf("negative slice %q value = %q, expected a visible negative amount", slice.Label, slice.ValStr)
			}
		}
	}
	if sum != 7_000 {
		t.Errorf("signed slices sum = %d, want spent 7000 EUR minor units", sum)
	}
	if !seenNegative {
		t.Fatal("the discount slice was not retained as a negative contribution")
	}
}

func TestBQ10BTaskRowUsesGeneratedTitleKeyAndArgs(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  string
		want string
	}{
		{name: "over limit", key: "tasks.budgetOverLimit", want: "Budget over limit: Groceries"},
		{name: "savings shortfall", key: "tasks.budgetSavingShortfall", want: "Savings target needs funding: Groceries"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := taskRowDisplayTitle(domain.Task{Title: "stored fallback", Custom: map[string]any{"titleKey": tc.key, "titleArgs": []any{"Groceries"}}})
			if got != uistate.T(tc.key, "Groceries") || got != tc.want {
				t.Fatalf("localized task title = %q, want %q", got, tc.want)
			}
		})
	}
	if got := taskRowDisplayTitle(domain.Task{Title: "Legacy title"}); got != "Legacy title" {
		t.Fatalf("legacy task title = %q, want stored fallback", got)
	}
}

func TestBQ10BGoalFundedBudgetSummaryUsesDescendantsAndPerLineOwner(t *testing.T) {
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	b := domain.Budget{ID: "home", CategoryID: "home", Scope: domain.ScopeIndividual, OwnerID: "alice"}
	txn := domain.Transaction{
		ID: "split", Date: start.AddDate(0, 0, 5), MemberID: "bob", GoalID: "goal",
		Amount: money.New(-60_000, "USD"),
		Splits: []domain.CategorySplit{
			{CategoryID: "repairs", Amount: money.New(-10_000, "USD"), MemberID: "alice"},
			{CategoryID: "repairs", Amount: money.New(-50_000, "USD"), MemberID: "bob"},
		},
	}
	categories := []domain.Category{{ID: "home", Name: "Home"}, {ID: "repairs", Name: "Repairs", ParentID: "home"}}
	summary, err := budgetGoalFundedSummary(b, []domain.Transaction{txn}, categories, start, end, currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	want := goalfunded.Split{FundedMinor: 10_000, TotalMinor: 10_000, Goals: []string{"goal"}}
	if summary.FundedMinor != want.FundedMinor || summary.TotalMinor != want.TotalMinor || len(summary.Goals) != 1 || summary.Goals[0] != "goal" {
		t.Fatalf("goal-funded summary = %+v, want Alice's $100 descendant-category contribution only", summary)
	}
	if over, rescued := summary.OverBy(9_000); over != 0 || !rescued {
		t.Fatalf("OverBy with same-currency limit = (%d, %t), want (0, true)", over, rescued)
	}
	ordinaryOver, err := budgetGoalFundedOverBy(goalfunded.Split{OrdinaryMinor: 250, FundedMinor: 100, TotalMinor: 350}, money.New(100, "EUR"), currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}})
	if err != nil || ordinaryOver != money.New(50, "USD") {
		t.Fatalf("mixed-currency ordinary overage = %v, %v; want 50 USD minor units", ordinaryOver, err)
	}
}
