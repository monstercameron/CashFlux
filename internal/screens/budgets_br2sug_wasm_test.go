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
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
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
	for i, amount := range []int64{120000, 130000, 90000} {
		date := monthStart.AddDate(0, -(i + 1), 10)
		br2sugPutExpense(t, app, "history-"+date.Format("2006-01"), date, -amount)
	}
	// Keep the projected month over the current $1,000 limit while below the
	// $1,200 historical median, so both the raise gate and projection floor are
	// exercised without making projection the source of the recommendation.
	daysInMonth := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Day()
	currentSpend := int64(110000 * now.Day() / daysInMonth)
	br2sugPutExpense(t, app, "current-pace", now, -currentSpend)
	rec, shown, ok := budgetRecommendationAt(app, budget, monthStart)
	if !ok {
		t.Fatal("expected a history-backed recommendation")
	}
	if rec.Direction != budgeting.SuggestionRaise {
		t.Fatalf("direction = %q, want raise after two comparable overspends", rec.Direction)
	}
	if rec.Suggested < rec.Projection {
		t.Fatalf("suggested monthly limit %d is below monthly projection %d", rec.Suggested, rec.Projection)
	}
	if rec.Suggested != 120000 {
		t.Fatalf("suggested monthly limit = %d, want historical median 120000", rec.Suggested)
	}
	if shown.Amount <= 0 {
		t.Fatal("converted row limit is not positive")
	}
}

func TestBR2SUGOnPaceBudgetDoesNotGetRaiseFromOneOverspend(t *testing.T) {
	app := br2sugTestApp(t)
	now := time.Now()
	budget := domain.Budget{ID: "transport-on-pace", Name: "Transportation", CategoryID: "transport-cat", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Period: domain.PeriodMonthly, Limit: money.New(100000, "USD")}
	if err := app.PutBudget(budget); err != nil {
		t.Fatalf("PutBudget: %v", err)
	}
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	for i, amount := range []int64{120000, 90000, 80000} {
		date := monthStart.AddDate(0, -(i + 1), 10)
		br2sugPutExpense(t, app, "on-pace-history-"+date.Format("2006-01"), date, -amount)
	}
	br2sugPutExpense(t, app, "on-pace-current", now, -10000)
	rec, _, ok := budgetRecommendationAt(app, budget, monthStart)
	if !ok {
		t.Fatal("expected a history-backed recommendation")
	}
	if rec.Direction == budgeting.SuggestionRaise {
		t.Fatalf("on-pace budget got a raise from only one comparable overspend: %+v", rec)
	}
}

func TestBR2SUGProjectionIncludesScheduledRecurringSpend(t *testing.T) {
	app := br2sugTestApp(t)
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	budget := domain.Budget{ID: "recurring-transport", Name: "Transportation", CategoryID: "transport-cat", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Period: domain.PeriodMonthly, Limit: money.New(100000, "USD")}
	if err := app.PutBudget(budget); err != nil {
		t.Fatalf("PutBudget: %v", err)
	}
	for i, amount := range []int64{120000, 130000, 90000} {
		date := monthStart.AddDate(0, -(i + 1), 10)
		br2sugPutExpense(t, app, "recurring-history-"+date.Format("2006-01"), date, -amount)
	}
	if err := app.PutRecurring(domain.Recurring{ID: "scheduled-gas", Label: "Scheduled gas", CategoryID: "transport-cat",
		Amount: money.New(-125000, "USD"), Cadence: domain.CadenceMonthly, NextDue: monthStart.AddDate(0, 1, -1)}); err != nil {
		t.Fatalf("PutRecurring: %v", err)
	}
	rec, _, ok := budgetRecommendationAt(app, budget, monthStart)
	if !ok {
		t.Fatal("expected a history-backed recommendation")
	}
	if rec.Direction != budgeting.SuggestionRaise {
		t.Fatalf("scheduled recurring projection over the limit should allow a raise, got %q", rec.Direction)
	}
	if rec.Projection != 125000 {
		t.Fatalf("monthly projection gate = %d, want scheduled recurring spend 125000", rec.Projection)
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

func TestBR2SUGEvidencePluralizesOutlierCount(t *testing.T) {
	cases := []struct {
		name     string
		outliers int
		want     string
	}{
		{name: "none", outliers: 0, want: "Based on a 3-month median."},
		{name: "one", outliers: 1, want: "3-month median, 1 unusual month left out."},
		{name: "many", outliers: 2, want: "3-month median, 2 unusual months left out."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key := "sm12.reasonNoOutliers"
			if tc.outliers == 1 {
				key = "sm12.reasonOneOutlier"
			} else if tc.outliers > 1 {
				key = "sm12.reasonManyOutliers"
			}
			if got := budgetSuggestionEvidence(key, 3, tc.outliers); got != tc.want {
				t.Errorf("evidence = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBR2SUGEditRecommendationUsesTargetWording(t *testing.T) {
	cases := []struct {
		direction budgeting.SuggestionDirection
		amount    string
		outliers  int
		want      string
	}{
		{direction: budgeting.SuggestionRaise, amount: "$136.30", want: "Raise to $136.30 Based on a 3-month median."},
		{direction: budgeting.SuggestionLower, amount: "$126.42", outliers: 2, want: "Lower to $126.42 3-month median, 2 unusual months left out."},
	}
	for _, tc := range cases {
		t.Run(string(tc.direction), func(t *testing.T) {
			if got := budgetRecommendationCopy(tc.direction, tc.amount, 3, tc.outliers); got != tc.want {
				t.Errorf("recommendation copy = %q, want %q", got, tc.want)
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
