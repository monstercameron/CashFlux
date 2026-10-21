// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"reflect"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func primaryGoalAllocationFixture(t *testing.T, cur string) (*appstate.App, domain.Account, domain.Goal) {
	t.Helper()
	app := primaryAccountsGoalsApp(t)
	if err := app.PutSettings(store.Settings{BaseCurrency: "USD", FXRates: map[string]float64{"EUR": 2}}); err != nil {
		t.Fatal(err)
	}
	a := domain.Account{ID: "bank", Name: "Checking", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(10000, "USD"), OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}
	g := domain.Goal{ID: "goal", Name: "Trip", TargetAmount: money.New(10000, cur), CurrentAmount: money.Zero(cur), OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}
	if err := app.PutAccount(a); err != nil {
		t.Fatal(err)
	}
	if err := app.PutGoal(g); err != nil {
		t.Fatal(err)
	}
	return app, a, g
}

func TestPrimaryGoalAllocationPreservesForeignAndUnavailableOriginals(t *testing.T) {
	app, a, g := primaryGoalAllocationFixture(t, "EUR")
	archived := a
	archived.ID = "archived"
	archived.Name = "Old bank"
	archived.Archived = true
	if err := app.PutAccount(archived); err != nil {
		t.Fatal(err)
	}
	g.Notes = "Keep goal metadata"
	g.Allocations = []domain.GoalAllocation{{AccountID: a.ID, Amount: money.New(1000, "USD")}, {AccountID: archived.ID, Amount: money.New(200, "USD")}, {AccountID: "deleted", Amount: money.New(300, "GBP")}}
	if err := app.PutGoal(g); err != nil {
		t.Fatal(err)
	}
	closed := 0
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(GoalAllocateForm, GoalAllocateFormProps{GoalID: g.ID, OnDone: func() { closed++ }}))
	f.Stabilize()
	if field := byTestID(f, "input", "goal-alloc-bank"); field == nil || field.Attr("data-cf-value") != "5.00" {
		t.Fatalf("foreign seed was relabeled instead of converted: %s", f.Text())
	}
	for _, id := range []string{"archived", "deleted"} {
		if field := byTestID(f, "input", "goal-alloc-"+id); field == nil || !isDisabled(field) {
			t.Fatalf("unavailable original row missing/editable: %s", f.Text())
		}
	}
	if byTestID(f, "p", "goal-alloc-unavailable") == nil {
		t.Fatal("unavailable reservations not disclosed")
	}
	byTestID(f, "form", "goal-allocate-form").Submit()
	f.Stabilize()
	stored := app.Goals()[0]
	if closed != 1 || stored.Notes != g.Notes || !reflect.DeepEqual(stored.Allocations, g.Allocations) {
		t.Fatalf("no-op save changed originals: %+v closed=%d", stored, closed)
	}
}

func TestPrimaryGoalAllocationExplicitEditAndUnavailableRemoval(t *testing.T) {
	app, a, g := primaryGoalAllocationFixture(t, "EUR")
	g.Allocations = []domain.GoalAllocation{{AccountID: a.ID, Amount: money.New(1000, "USD")}, {AccountID: "deleted", Amount: money.New(300, "GBP")}}
	if err := app.PutGoal(g); err != nil {
		t.Fatal(err)
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(GoalAllocateForm, GoalAllocateFormProps{GoalID: g.ID}))
	f.Stabilize()
	byTestID(f, "input", "goal-alloc-bank").Input("10.00")
	f.Stabilize()
	byTestID(f, "input", "goal-alloc-pick-deleted").Change("on")
	f.Stabilize()
	byTestID(f, "form", "goal-allocate-form").Submit()
	f.Stabilize()
	stored := app.Goals()[0]
	if len(stored.Allocations) != 1 || stored.Allocations[0].Amount != money.New(1000, "EUR") {
		t.Fatalf("explicit replacement/removal wrong: %+v", stored.Allocations)
	}
}

func TestPrimaryGoalAllocationSaveRevalidatesLiveBalanceAndRates(t *testing.T) {
	for _, mode := range []string{"balance", "rates", "other-reservations"} {
		t.Run(mode, func(t *testing.T) {
			cur := "USD"
			value := "75.00"
			wantFree := "50.00"
			if mode == "rates" {
				cur = "EUR"
				value = "40.00"
				wantFree = "25.00"
			}
			app, a, g := primaryGoalAllocationFixture(t, cur)
			closed := 0
			f := render.New(t)
			defer f.Cleanup()
			f.Render(ui.CreateElement(GoalAllocateForm, GoalAllocateFormProps{GoalID: g.ID, OnDone: func() { closed++ }}))
			f.Stabilize()
			byTestID(f, "input", "goal-alloc-pick-bank").Change("on")
			f.Stabilize()
			byTestID(f, "input", "goal-alloc-bank").Input(value)
			f.Stabilize()
			// Deliberately omit BumpDataRevision: the submit handler must read the
			// latest store revision rather than trusting captured render data.
			switch mode {
			case "balance":
				a.OpeningBalance = money.New(5000, "USD")
				if err := app.PutAccount(a); err != nil {
					t.Fatal(err)
				}
			case "rates":
				if err := app.PutSettings(store.Settings{BaseCurrency: "USD", FXRates: map[string]float64{"EUR": 4}}); err != nil {
					t.Fatal(err)
				}
			default:
				other := g
				other.ID = "other"
				other.Name = "Other"
				other.Allocations = []domain.GoalAllocation{{AccountID: a.ID, Amount: money.New(5000, "USD")}}
				if err := app.PutGoal(other); err != nil {
					t.Fatal(err)
				}
			}
			byTestID(f, "form", "goal-allocate-form").Submit()
			f.Stabilize()
			if closed != 0 {
				t.Fatal("stale over-reservation closed the editor")
			}
			if note := byTestID(f, "p", "goal-alloc-error"); note == nil || !strings.Contains(note.Text(), wantFree) {
				t.Fatalf("live headroom not checked: %s", f.Text())
			}
			for _, stored := range app.Goals() {
				if stored.ID == g.ID && len(stored.Allocations) != 0 {
					t.Fatalf("stale amount persisted: %+v", stored.Allocations)
				}
			}
		})
	}
}

func TestPrimaryGoalAllocationMissingFXAndCurrencyChangeAreExplicit(t *testing.T) {
	app, _, g := primaryGoalAllocationFixture(t, "EUR")
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(GoalAllocateForm, GoalAllocateFormProps{GoalID: g.ID}))
	f.Stabilize()
	byTestID(f, "input", "goal-alloc-pick-bank").Change("on")
	f.Stabilize()
	byTestID(f, "input", "goal-alloc-bank").Input("10.00")
	f.Stabilize()
	g.TargetAmount = money.New(10000, "USD")
	g.CurrentAmount = money.Zero("USD")
	g.Notes = "Latest metadata"
	if err := app.PutGoal(g); err != nil {
		t.Fatal(err)
	}
	byTestID(f, "form", "goal-allocate-form").Submit()
	f.Stabilize()
	if note := byTestID(f, "p", "goal-alloc-error"); note == nil || !strings.Contains(note.Text(), "currency changed") {
		t.Fatalf("stale EUR text relabeled as USD: %s", f.Text())
	}
	if len(app.Goals()[0].Allocations) != 0 {
		t.Fatal("stale currency edit saved")
	}
	byTestID(f, "input", "goal-alloc-bank").Input("10.00")
	f.Stabilize()
	byTestID(f, "form", "goal-allocate-form").Submit()
	f.Stabilize()
	stored := app.Goals()[0]
	if stored.Notes != "Latest metadata" || len(stored.Allocations) != 1 || stored.Allocations[0].Amount != money.New(1000, "USD") {
		t.Fatalf("reviewed edit or live metadata lost: %+v", stored)
	}
	// The rate table disappears while a foreign goal's editor is visible.
	g.TargetAmount = money.New(10000, "EUR")
	g.CurrentAmount = money.Zero("EUR")
	g.Allocations = stored.Allocations
	if err := app.PutGoal(g); err != nil {
		t.Fatal(err)
	}
	if err := app.PutSettings(store.Settings{BaseCurrency: "USD"}); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if byTestID(f, "p", "goal-alloc-unavailable") == nil {
		t.Fatal("missing FX disclosure absent")
	}
	if field := byTestID(f, "input", "goal-alloc-bank"); field == nil || !isDisabled(field) {
		t.Fatalf("missing FX source remains editable: %s", f.Text())
	}
}
