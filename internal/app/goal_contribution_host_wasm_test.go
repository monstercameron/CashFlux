// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"io"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/screens"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func contributionHostFixture() ui.Node {
	_ = uistate.UseNotice().Get()
	return ui.Fragment(ui.CreateElement(screens.Goals), ui.CreateElement(GoalEditHost))
}

func contributionHostApp(t *testing.T, goal domain.Goal) (*appstate.App, state.GlobalAtom[uistate.GoalEdit], state.GlobalAtom[uistate.Notice]) {
	t.Helper()
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	previousApp := appstate.Default
	appstate.Default = app
	edit := state.NewGlobalAtom("goals:edit", uistate.GoalEdit{})
	previousEdit := edit.Get()
	edit.Set(uistate.GoalEdit{ID: goal.ID, Mode: uistate.GoalEditModeContribute})
	notice := state.NewGlobalAtom("app:notice", uistate.Notice{})
	previousNotice := notice.Get()
	notice.Set(uistate.Notice{})
	view := state.NewGlobalAtom("goals:view", uistate.GoalsViewGoals)
	previousView := view.Get()
	view.Set(uistate.GoalsViewGoals)
	lens := state.NewGlobalAtom("household:activeScope", scope.ReportScope{})
	previousLens := lens.Get()
	lens.Set(scope.ReportScope{})
	t.Cleanup(func() {
		appstate.Default = previousApp
		edit.Set(previousEdit)
		notice.Set(previousNotice)
		view.Set(previousView)
		lens.Set(previousLens)
		_ = app.Store().Close()
	})
	if err := app.PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	return app, edit, notice
}

func contributionHostGoal(saved int64) domain.Goal {
	return domain.Goal{ID: "contribution", Name: "Trip", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		CurrentAmount: money.New(saved, "USD"), TargetAmount: money.New(1000, "USD"), AccountID: "bank"}
}

func renderContributionHost(fixture *render.Fixture, goalID string) {
	fixture.Render(ui.CreateElement(contributionHostFixture))
	fixture.Stabilize()
	// render.New installs the fixture's runtime. Open through the production
	// selector after its host captures that runtime's atom, as a card action does.
	uistate.SetGoalEdit(uistate.GoalEdit{ID: goalID, Mode: uistate.GoalEditModeContribute})
	fixture.Stabilize()
}

func contributionHostInput(t *testing.T, fixture *render.Fixture, goalID, amount string, postLedger bool) {
	t.Helper()
	var amountInput, checkbox *render.QueryNode
	for _, input := range fixture.AllByTag("input") {
		switch input.Attr("id") {
		case "goal-contrib-" + goalID:
			amountInput = input
		case "goal-contrib-ledger-" + goalID:
			checkbox = input
		}
	}
	if amountInput == nil {
		t.Fatalf("actual contribution form did not mount: %s", fixture.Text())
	}
	amountInput.Input(amount)
	fixture.Stabilize()
	if postLedger {
		if checkbox == nil {
			t.Fatal("linked account debit option is absent")
		}
		checkbox.Dispatch("onchange", render.Event{Checked: true})
		fixture.Stabilize()
	}
}

func contributionHostForm(t *testing.T, fixture *render.Fixture) *render.QueryNode {
	t.Helper()
	for _, form := range fixture.AllByTag("form") {
		if strings.Contains(form.Attr("class"), "goal-contribute") {
			return form
		}
	}
	t.Fatalf("actual contribution form is absent: %s", fixture.Text())
	return nil
}

func TestGoalContributionHostUsesLatestProgressMetadataAndNativeCompletion(t *testing.T) {
	goal := contributionHostGoal(100)
	goal.Allocations = []domain.GoalAllocation{{AccountID: "reserved", Amount: money.New(400, "EUR")}}
	app, edit, notice := contributionHostApp(t, goal)
	if err := app.PutSettings(store.Settings{BaseCurrency: "USD", FXRates: map[string]float64{"EUR": 2}}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"bank", "new-bank"} {
		if err := app.PutAccount(domain.Account{ID: id, Name: id, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
			Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(10000, "USD")}); err != nil {
			t.Fatal(err)
		}
	}
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	renderContributionHost(f, goal.ID)
	contributionHostInput(t, f, goal.ID, "1.00", true)
	// Mutate externally after typing without a UI notification: the mounted form
	// must preserve the new metadata, saved progress, log and eligible primary link.
	latest := goal
	latest.Name, latest.Notes, latest.Priority, latest.AccountID = "Updated trip", "Keep current notes", 4, "new-bank"
	latest.CurrentAmount = money.New(150, "USD")
	latest = latest.RecordContribution(domain.GoalContribution{Amount: money.New(50, "USD")})
	if err := app.PutGoal(latest); err != nil {
		t.Fatal(err)
	}
	contributionHostForm(t, f).Submit()
	f.Stabilize()
	stored := app.Goals()[0]
	if stored.Name != latest.Name || stored.Notes != latest.Notes || stored.Priority != latest.Priority || stored.AccountID != latest.AccountID || stored.CurrentAmount.Amount != 250 || len(stored.Contributions) != 2 || !reflect.DeepEqual(stored.Allocations, goal.Allocations) {
		t.Fatalf("mounted contribution overwrote current state: %+v", stored)
	}
	if txns := app.Transactions(); len(txns) != 1 || txns[0].AccountID != latest.AccountID || txns[0].Amount != money.New(-100, "USD") {
		t.Fatalf("stale linked account determined the debit: %+v", txns)
	}
	if edit.Get().ID != "" || notice.Get().Text != uistate.T("goals.completionPrompt") {
		t.Fatalf("canonical foreign-reservation completion did not close/prompt: edit=%+v notice=%+v", edit.Get(), notice.Get())
	}
}

func TestGoalContributionHostUnknownFXNeverAnnouncesReached(t *testing.T) {
	goal := contributionHostGoal(950)
	goal.Allocations = []domain.GoalAllocation{{AccountID: "reserved", Amount: money.New(2000, "EUR")}}
	app, edit, notice := contributionHostApp(t, goal)
	if err := app.PutSettings(store.Settings{BaseCurrency: "USD"}); err != nil {
		t.Fatal(err)
	}
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	renderContributionHost(f, goal.ID)
	contributionHostInput(t, f, goal.ID, "0.50", false)
	contributionHostForm(t, f).Submit()
	f.Stabilize()
	if app.Goals()[0].CurrentAmount.Amount != 1000 || edit.Get().ID != "" {
		t.Fatalf("known saved contribution failed: %s", f.Text())
	}
	if n := notice.Get(); n.Seq != 1 || n.Text == uistate.T("goals.milestone100") || n.Text == uistate.T("goals.completionPrompt") || !n.Undoable {
		t.Fatalf("unknown coverage announced a reached/completed goal: %+v", n)
	}
	if findAppTestID(f, "p", "goal-coverage-unavailable-"+goal.ID) == nil {
		t.Fatalf("actual goal card hid unavailable coverage: %s", f.Text())
	}
}

func TestGoalContributionHostOverflowDeletedGoalAndCurrencyConflictDoNotMutate(t *testing.T) {
	for _, scenario := range []string{"overflow", "deleted", "currency changed"} {
		t.Run(scenario, func(t *testing.T) {
			goal := contributionHostGoal(100)
			if scenario == "overflow" {
				goal.CurrentAmount, goal.TargetAmount = money.New(math.MaxInt64, "USD"), money.New(math.MaxInt64, "USD")
			}
			app, edit, notice := contributionHostApp(t, goal)
			if err := app.PutAccount(domain.Account{ID: "bank", Name: "Bank", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
				Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"}); err != nil {
				t.Fatal(err)
			}
			f := render.New(t, render.WithQueuedScheduler())
			defer f.Cleanup()
			renderContributionHost(f, goal.ID)
			contributionHostInput(t, f, goal.ID, "0.01", true)
			if scenario == "deleted" {
				if err := app.DeleteGoal(goal.ID); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "currency changed" {
				goal.CurrentAmount, goal.TargetAmount = money.New(100, "EUR"), money.New(1000, "EUR")
				if err := app.PutGoal(goal); err != nil {
					t.Fatal(err)
				}
			}
			before, revision := app.Goals(), app.Rev()
			contributionHostForm(t, f).Submit()
			f.Stabilize()
			if !reflect.DeepEqual(app.Goals(), before) || app.Rev() != revision || len(app.Transactions()) != 0 || notice.Get().Seq != 0 || edit.Get().ID != goal.ID {
				t.Fatalf("rejected %s contribution changed goal/ledger/notice/modal: %s", scenario, f.Text())
			}
		})
	}
}
