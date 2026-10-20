// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestPrimaryGoalSummaryUsesConvertedTotalsAndDisclosesExclusions(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	settings := store.Settings{BaseCurrency: "USD", FXRates: map[string]float64{"EUR": 2}}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	for _, g := range []domain.Goal{
		{ID: "usd", Name: "USD", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, CurrentAmount: money.New(10000, "USD"), TargetAmount: money.New(10000, "USD")},
		{ID: "eur", Name: "EUR", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, CurrentAmount: money.New(0, "EUR"), TargetAmount: money.New(10000, "EUR")},
		{ID: "gbp", Name: "GBP", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, CurrentAmount: money.New(100000, "GBP"), TargetAmount: money.New(100000, "GBP")},
	} {
		if err := app.PutGoal(g); err != nil {
			t.Fatal(err)
		}
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(goalSummaryWidget, goalSummaryProps{App: app}))
	f.Stabilize()
	if text := f.Text(); !strings.Contains(text, "33%") || !strings.Contains(text, "300.00") {
		t.Fatalf("summary does not reconcile with converted totals: %s", text)
	}
	note := byTestID(f, "p", "goal-summary-excluded-currencies")
	if note == nil || !strings.Contains(note.Text(), "GBP") {
		t.Fatalf("missing-FX omission is not disclosed: %v", note)
	}
	revision := app.Rev()
	settings.FXRates["GBP"] = 1
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	if app.Rev() == revision {
		t.Fatal("settings write did not advance the store revision")
	}
	// Verify the model's store-revision invalidation independently of rendering.
	updated := computeGoalView(app, "")
	if len(updated.ExcludedCurrencies) != 0 || updated.TargetTotal.Amount != 130000 || updated.OverallPct != 84 {
		t.Fatalf("settings revision retained a stale goal model: %+v", updated)
	}
	// Real settings handlers notify subscribers after saving. Re-rendering the
	// same component with the same App pointer can be skipped by reconciliation.
	uistate.BumpDataRevision()
	f.Stabilize()
	if byTestID(f, "p", "goal-summary-excluded-currencies") != nil {
		t.Fatal("excluded currency note survived the settings revision")
	}
	if text := f.Text(); !strings.Contains(text, "84%") || !strings.Contains(text, "1,300.00") {
		t.Fatalf("summary did not update after the settings notification: %s", text)
	}
}

func TestPrimaryGoalQuickFundingRechecksBalanceBeforeClick(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	a := domain.Account{ID: "bank", Name: "Checking", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(10000, "USD")}
	if err := app.PutAccount(a); err != nil {
		t.Fatal(err)
	}
	g := domain.Goal{ID: "goal", Name: "Trip", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, CurrentAmount: money.Zero("USD"), TargetAmount: money.New(15000, "USD"), Allocations: []domain.GoalAllocation{{AccountID: a.ID, Amount: money.New(3000, "USD")}}}
	if err := app.PutGoal(g); err != nil {
		t.Fatal(err)
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(func(struct{}) ui.Node {
		_ = uistate.UseDataRevision().Get()
		return ui.CreateElement(GoalRow, goalRowProps{Goal: app.Goals()[0], Accounts: app.Accounts(), Base: "USD"})
	}, struct{}{}))
	f.Stabilize()
	expand := byTestID(f, "button", "goal-expand-"+g.ID)
	if expand == nil {
		t.Fatal("missing goal details control")
	}
	expand.Click()
	f.Stabilize()
	quick := byTestID(f, "button", "goal-quickfund-"+g.ID)
	if quick == nil || !strings.Contains(quick.Text(), "70.00") {
		t.Fatalf("quick funding ignored existing reservation: %s", f.Text())
	}
	// Leave the rendered suggestion stale: the callback must read the new store
	// revision before adding money, even without a browser notification first.
	a.OpeningBalance = money.New(5000, "USD")
	if err := app.PutAccount(a); err != nil {
		t.Fatal(err)
	}
	quick.Click()
	f.Stabilize()
	updated := app.Goals()[0]
	if updated.AllocatedMinor() != 5000 {
		t.Fatalf("stale click over-reserved bank balance: %+v", updated.Allocations)
	}
	if _, _, amount := bestQuickEarmark(app, g.ID); amount != 0 {
		t.Fatalf("fully reserved account still offers %d", amount)
	}
}

func TestPrimaryGoalEarmarksFXRevisionAndRemove(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	settings := store.Settings{BaseCurrency: "USD"}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	a := domain.Account{ID: "bank", Name: "Checking", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(10000, "USD")}
	if err := app.PutAccount(a); err != nil {
		t.Fatal(err)
	}
	for _, g := range []domain.Goal{
		{ID: "usd", Name: "USD", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, CurrentAmount: money.Zero("USD"), TargetAmount: money.New(10000, "USD"), Allocations: []domain.GoalAllocation{{AccountID: a.ID, Amount: money.New(2000, "USD")}}},
		{ID: "eur", Name: "EUR", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, CurrentAmount: money.Zero("EUR"), TargetAmount: money.New(10000, "EUR"), Allocations: []domain.GoalAllocation{{AccountID: a.ID, Amount: money.New(1000, "EUR")}}},
	} {
		if err := app.PutGoal(g); err != nil {
			t.Fatal(err)
		}
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(goalEarmarksManager, goalEarmarksProps{App: app}))
	f.Stabilize()
	if note := byTestID(f, "p", "earmarks-excluded"); note == nil || !strings.Contains(note.Text(), "EUR") {
		t.Fatalf("missing reservation FX is not disclosed: %s", f.Text())
	}
	settings.FXRates = map[string]float64{"EUR": 2}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if byTestID(f, "p", "earmarks-excluded") != nil {
		t.Fatal("FX revision did not clear exclusion")
	}
	if exposure := byTestID(f, "div", "ea-exp-"+a.ID); exposure == nil || !strings.Contains(exposure.Text(), "60.00") {
		t.Fatalf("exposure did not use converted reservations: %s", f.Text())
	}
	remove := byTestID(f, "button", "ea-del-eur-"+a.ID)
	if remove == nil {
		t.Fatal("missing earmark removal control")
	}
	remove.Click()
	f.Stabilize()
	if got := goalFunding(app).Earmarks.Free.Amount; got != 8000 {
		t.Fatalf("removed earmark retained headroom: %d", got)
	}
	if exposure := byTestID(f, "div", "ea-exp-"+a.ID); exposure == nil || !strings.Contains(exposure.Text(), "80.00") {
		t.Fatalf("removed reservation retained stale exposure: %s", f.Text())
	}
}

func TestPrimaryGoalFundScopeDisclosesMissingFX(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	if err := app.PutSettings(store.Settings{BaseCurrency: "USD"}); err != nil {
		t.Fatal(err)
	}
	for _, g := range []domain.Goal{
		{ID: "usd", Name: "USD fund", IsSinkingFund: true, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, CurrentAmount: money.Zero("USD"), TargetAmount: money.New(10000, "USD")},
		{ID: "gbp", Name: "GBP fund", IsSinkingFund: true, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, CurrentAmount: money.Zero("GBP"), TargetAmount: money.New(100000, "GBP")},
	} {
		if err := app.PutGoal(g); err != nil {
			t.Fatal(err)
		}
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(goalSummaryWidget, goalSummaryProps{App: app}))
	f.Stabilize()
	if note := byTestID(f, "span", "goal-funds-excluded"); note == nil || !strings.Contains(note.Text(), "GBP") {
		t.Fatalf("fund FX omission hidden: %s", f.Text())
	}
	if scope := byTestID(f, "p", "goal-headline-scope"); scope == nil || !strings.Contains(scope.Text(), "100.00") {
		t.Fatalf("unknown foreign target treated as base: %s", f.Text())
	}
	if err := app.PutSettings(store.Settings{BaseCurrency: "USD", FXRates: map[string]float64{"GBP": 2}}); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if byTestID(f, "span", "goal-funds-excluded") != nil {
		t.Fatal("fund exclusion survived FX revision")
	}
	if scope := byTestID(f, "p", "goal-headline-scope"); scope == nil || !strings.Contains(scope.Text(), "2,100.00") {
		t.Fatalf("fund target did not update: %s", f.Text())
	}
}
