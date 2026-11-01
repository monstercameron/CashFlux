// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/untracked"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestUntrackedApplyPlanCreatesOwnedBudgetAndRaisesAtDestinationCadence(t *testing.T) {
	candidates := []untracked.Candidate{
		{CategoryID: "annual", Name: "Annual bill", SuggestMinor: 120000, Period: domain.PeriodYearly},
		{CategoryID: "other", Name: "Other", SuggestMinor: 15000, Period: domain.PeriodYearly},
	}
	rows := map[string]trackRow{
		"annual": {Include: true, Amount: 120000, Raise: true},
		"other":  {Include: true, Amount: 15000, BudgetID: "eur-monthly", Raise: true},
	}
	budgets := []domain.Budget{{
		ID: "eur-monthly", Name: "Travel", CategoryIDs: []string{"travel"}, Period: domain.PeriodMonthly,
		Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Limit: money.New(5000, "EUR"),
	}}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 0.8}}

	creates, updates, err := planTrackUntracked(candidates, rows, budgets, "USD", rates, "member-1")
	if err != nil {
		t.Fatalf("planTrackUntracked: %v", err)
	}
	if len(creates) != 1 {
		t.Fatalf("creates = %d, want one", len(creates))
	}
	created := creates[0]
	if created.Scope != domain.ScopeIndividual || created.OwnerID != "member-1" || created.Period != domain.PeriodYearly || created.Limit != money.New(120000, "USD") {
		t.Errorf("created budget = %+v, want member-owned yearly USD budget", created)
	}
	if len(updates) != 1 {
		t.Fatalf("updates = %d, want one", len(updates))
	}
	updated := updates[0]
	if updated.Limit != money.New(6563, "EUR") {
		t.Errorf("raised EUR monthly limit = %v, want 6563 EUR minor units (€65.63)", updated.Limit)
	}
	if got := updated.TrackedCategoryIDs(); len(got) != 2 || got[1] != "other" {
		t.Errorf("tracked categories = %v, want existing travel plus other", got)
	}
}

func TestUntrackedApplyPlanRejectsStaleDestinationBeforeWriting(t *testing.T) {
	candidates := []untracked.Candidate{{CategoryID: "auto", Name: "Auto", SuggestMinor: 10000, Period: domain.PeriodMonthly}}
	rows := map[string]trackRow{"auto": {Include: true, Amount: 10000, BudgetID: "deleted", Raise: true}}
	_, _, err := planTrackUntracked(candidates, rows, nil, "USD", currency.Rates{Base: "USD"}, "")
	if err == nil {
		t.Fatal("planTrackUntracked accepted a destination that no longer exists")
	}
}

func TestUntrackedApplyWritesNewAndRaisedBudgetsWithHistory(t *testing.T) {
	app := guardTestApp(t)
	for _, category := range []domain.Category{
		{ID: "bc4b3-new", Name: "Annual bill", Kind: domain.KindExpense},
		{ID: "bc4b3-extra", Name: "Other", Kind: domain.KindExpense},
	} {
		if err := app.PutCategory(category); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.PutBudgetAt(domain.Budget{
		ID: "bc4b3-existing", Name: "Travel", CategoryID: "c-transport", Period: domain.PeriodMonthly,
		Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Limit: money.New(5000, "USD"),
	}, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	ref := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	candidates := []untracked.Candidate{
		{CategoryID: "bc4b3-new", Name: "Annual bill", SuggestMinor: 120000, Period: domain.PeriodYearly},
		{CategoryID: "bc4b3-extra", Name: "Other", SuggestMinor: 12000, Period: domain.PeriodYearly},
	}
	rows := putTrackRow(putTrackRow("", "bc4b3-new", trackRow{Include: true, Amount: 120000}),
		"bc4b3-extra", trackRow{Include: true, Amount: 12000, BudgetID: "bc4b3-existing", Raise: true})
	cleared, closed := false, false
	if err := applyTrackUntracked(app, candidates, "USD", currency.Rates{Base: "USD"}, "member-1", ref,
		func() string { return rows }, func(string) { cleared = true }, func() { closed = true }); err != nil {
		t.Fatalf("applyTrackUntracked: %v", err)
	}
	if !cleared || !closed {
		t.Fatalf("apply completion callbacks cleared=%t closed=%t", cleared, closed)
	}
	var created, updated *domain.Budget
	for _, budget := range app.Budgets() {
		b := budget
		switch b.CategoryID {
		case "bc4b3-new":
			created = &b
		case "c-transport":
			updated = &b
		}
	}
	if created == nil || created.OwnerID != "member-1" || created.Scope != domain.ScopeIndividual || created.Period != domain.PeriodYearly {
		t.Fatalf("created budget = %+v, want member-owned yearly budget", created)
	}
	if updated == nil || updated.Limit.Amount != 6000 || len(updated.TrackedCategoryIDs()) != 2 {
		t.Fatalf("updated budget = %+v, want 6000 monthly with second tracked category", updated)
	}
	if created.LimitHistory["2026-01-01"] != 120000 || updated.LimitHistory["2026-10-01"] != 6000 {
		t.Fatalf("history create=%v update=%v, want effective October limits", created.LimitHistory, updated.LimitHistory)
	}
}

func TestFlexShowsReadableErrorWhenCurrencyCannotBeConverted(t *testing.T) {
	app := guardTestApp(t)
	if err := app.PutTransaction(domain.Transaction{
		ID: "bc4b3-unpriced", AccountID: "a", CategoryID: "c-gas", Desc: "Unpriced purchase", Date: time.Date(budgetNow().Year(), budgetNow().Month(), 10, 0, 0, 0, 0, time.UTC),
		Amount: money.New(-5000, "EUR"),
	}); err != nil {
		t.Fatal(err)
	}
	f := render.New(t)
	f.Render(ui.CreateElement(budgetFlexWidget, budgetSummaryProps{App: app}))
	if got := f.Text(); !strings.Contains(got, "I couldn't calculate flex spending") {
		t.Fatalf("flex error copy = %q, want a plain-language calculation failure", got)
	}
}
