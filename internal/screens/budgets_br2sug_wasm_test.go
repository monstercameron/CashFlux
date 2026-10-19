// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestBR2SUGOverPaceRecommendationDoesNotCutBelowProjection(t *testing.T) {
	app := br2sugTestApp(t)
	now := time.Now()
	budget := domain.Budget{ID: "transport", Name: "Transportation", CategoryID: "transport-cat", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Period: domain.PeriodMonthly, Limit: money.New(100000, "USD")}
	if err := app.PutBudget(budget); err != nil {
		t.Fatalf("PutBudget: %v", err)
	}
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	for i := 1; i <= 3; i++ {
		date := monthStart.AddDate(0, -i, 10)
		br2sugPutExpense(t, app, "history-"+date.Format("2006-01"), date, -50000)
	}
	br2sugPutExpense(t, app, "current-pace", now, -300000)
	rec, shown, ok := budgetRecommendationAt(app, budget, monthStart)
	if !ok {
		t.Fatal("expected a history-backed recommendation")
	}
	if rec.Direction != budgeting.SuggestionRaise {
		t.Fatalf("direction = %q, want raise", rec.Direction)
	}
	if rec.Suggested < rec.Projection {
		t.Fatalf("suggested monthly limit %d is below monthly projection %d", rec.Suggested, rec.Projection)
	}
	if shown.Amount <= 0 {
		t.Fatal("converted row limit is not positive")
	}
}

func TestBR2SUGZeroHistoryHasNoSuggestion(t *testing.T) {
	app := br2sugTestApp(t)
	budget := domain.Budget{ID: "empty", Name: "Subscriptions", CategoryID: "transport-cat", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Period: domain.PeriodMonthly, Limit: money.New(4000, "USD")}
	if err := app.PutBudget(budget); err != nil {
		t.Fatalf("PutBudget: %v", err)
	}
	if _, _, ok := budgetRecommendationAt(app, budget, time.Now()); ok {
		t.Fatal("zero-history budget got a suggestion")
	}
}

func TestBR2SUGDirectionalCopyAndAccessibleNames(t *testing.T) {
	cases := []struct {
		direction           budgeting.SuggestionDirection
		visible, accessible string
	}{
		{budgeting.SuggestionRaise, "Raise the Subscriptions limit to $136.30", "Raise the Subscriptions limit to $136.30"},
		{budgeting.SuggestionLower, "Lower the Transportation limit to $126.42", "Lower the Transportation limit to $126.42"},
	}
	for _, tc := range cases {
		t.Run(string(tc.direction), func(t *testing.T) {
			verbKey, verb := "sm12.raise", uistate.T("sm12.raise")
			if tc.direction == budgeting.SuggestionLower {
				verbKey, verb = "sm12.lower", uistate.T("sm12.lower")
			}
			name := "Subscriptions"
			amount := "$136.30"
			if tc.direction == budgeting.SuggestionLower {
				name, amount = "Transportation", "$126.42"
			}
			props := budgetSuggestRowHintProps{Key: "test-" + string(tc.direction), TestID: "br2sug", Text: uistate.T("sm12.directionalHint", verb, amount),
				ActionLabel: uistate.T(verbKey+"AccessibleAction", name, amount), AccessibleActionLabel: uistate.T(verbKey+"AccessibleAction", name, amount)}
			f := render.New(t)
			f.Render(ui.CreateElement(budgetSuggestRowHintInner, props))
			button := byTestID(f, "button", "smart-rowhint-act-br2sug")
			if button == nil {
				t.Fatal("directional suggestion action did not render")
			}
			if got := button.Text(); got != tc.visible {
				t.Errorf("visible action = %q, want %q", got, tc.visible)
			}
			if got := button.Attr("aria-label"); got != tc.accessible {
				t.Errorf("accessible action = %q, want %q", got, tc.accessible)
			}
		})
	}
}

func br2sugTestApp(t *testing.T) *appstate.App {
	t.Helper()
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("appstate.New: %v", err)
	}
	if err := app.PutCategory(domain.Category{ID: "transport-cat", Name: "Transportation", Kind: domain.KindExpense}); err != nil {
		t.Fatalf("PutCategory: %v", err)
	}
	if err := app.PutAccount(domain.Account{ID: "br2sug-account", Name: "Checking", Currency: "USD", Type: domain.TypeChecking,
		Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	previous := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = previous })
	return app
}

func br2sugPutExpense(t *testing.T, app *appstate.App, id string, date time.Time, amount int64) {
	t.Helper()
	if err := app.PutTransaction(domain.Transaction{ID: id, AccountID: "br2sug-account", Desc: "Test expense", Date: date,
		CategoryID: "transport-cat", Amount: money.New(amount, "USD")}); err != nil {
		t.Fatalf("PutTransaction %s: %v", id, err)
	}
}
