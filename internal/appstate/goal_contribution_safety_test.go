// SPDX-License-Identifier: MIT

package appstate

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

func contributionSafetyGoal(current int64) domain.Goal {
	return domain.Goal{ID: "goal", Name: "Trip", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		TargetAmount: money.New(1000, "USD"), CurrentAmount: money.New(current, "USD"), AccountID: "bank"}
}

func TestContributeRejectsUnsafeAmountsBeforeGoalOrLedgerMutation(t *testing.T) {
	for _, test := range []struct {
		name string
		goal domain.Goal
		amt  money.Money
	}{
		{"zero", contributionSafetyGoal(0), money.Zero("USD")},
		{"negative", contributionSafetyGoal(0), money.New(-1, "USD")},
		{"overflow", contributionSafetyGoal(math.MaxInt64), money.New(1, "USD")},
		{"currency mismatch", contributionSafetyGoal(100), money.New(1, "EUR")},
		{"unspecified contribution currency", contributionSafetyGoal(0), money.New(1, "")},
	} {
		t.Run(test.name, func(t *testing.T) {
			app := newApp(t, false)
			defer app.Store().Close()
			if err := app.Store().PutGoal(test.goal); err != nil {
				t.Fatal(err)
			}
			before := findGoal(t, app, test.goal.ID)
			revision := app.Rev()
			if _, err := app.ContributeToGoal(test.goal, test.amt, true); err == nil {
				t.Fatal("unsafe contribution succeeded")
			}
			if app.Rev() != revision || !reflect.DeepEqual(findGoal(t, app, test.goal.ID), before) || len(app.Transactions()) != 0 {
				t.Fatal("failed contribution changed the goal or ledger")
			}
		})
	}
	for _, current := range []int64{0, math.MaxInt64 - 1} {
		app := newApp(t, false)
		defer app.Store().Close()
		goal := contributionSafetyGoal(current)
		goal.TargetAmount = money.New(math.MaxInt64, "USD")
		if err := app.PutGoal(goal); err != nil {
			t.Fatal(err)
		}
		amount := int64(math.MaxInt64) - current
		result, err := app.ContributeToGoal(goal, money.New(amount, "USD"), false)
		if err != nil || findGoal(t, app, goal.ID).CurrentAmount.Amount != math.MaxInt64 || !result.BecameComplete || result.AfterSavedPercent != 100 {
			t.Fatalf("representable positive boundary was rejected: %+v %v", result, err)
		}
	}
}

func TestContributePreflightsLegacySavedCurrencyAndPreservesLedgerFailureContract(t *testing.T) {
	app := newApp(t, false)
	defer app.Store().Close()
	goal := contributionSafetyGoal(100)
	goal.CurrentAmount = money.New(100, "EUR")
	if err := app.Store().PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	if err := app.PutSettings(store.Settings{BaseCurrency: "USD", FXRates: map[string]float64{"EUR": 2}}); err != nil {
		t.Fatal(err)
	}
	if err := app.PutAccount(domain.Account{ID: "bank", Name: "Bank", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "EUR"}); err != nil {
		t.Fatal(err)
	}
	revision := app.Rev()
	if _, err := app.ContributeToGoal(goal, money.New(100, "EUR"), true); err == nil {
		t.Fatal("unsupported saved/target currency mismatch was persisted")
	}
	if app.Rev() != revision || len(app.Transactions()) != 0 || !reflect.DeepEqual(findGoal(t, app, goal.ID), goal) {
		t.Fatal("goal validation failure left a stray debit or normalized legacy money")
	}
	goal = contributionSafetyGoal(0)
	if err := app.PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	result, err := app.ContributeToGoalFrom(goal, money.New(500, "USD"), true, "missing-account")
	if err == nil || result.TransactionID != "" || result.Milestone != 50 {
		t.Fatalf("ledger failure result contract changed: %+v %v", result, err)
	}
	stored := findGoal(t, app, goal.ID)
	if stored.CurrentAmount.Amount != 500 || len(stored.Contributions) != 1 || stored.Contributions[0].TxnID != "" || len(app.Transactions()) != 0 {
		t.Fatal("ledger failure no longer preserves a memo-only saved contribution")
	}
}

func TestContributeUsesLatestGoalMetadataProgressAndPrimaryAccount(t *testing.T) {
	app := newApp(t, false)
	defer app.Store().Close()
	stale := contributionSafetyGoal(100)
	if err := app.PutGoal(stale); err != nil {
		t.Fatal(err)
	}
	latest := stale
	latest.Name, latest.Notes, latest.AccountID, latest.Priority = "Updated trip", "Keep me", "new-bank", 3
	latest.TargetAmount, latest.CurrentAmount = money.New(2000, "USD"), money.New(700, "USD")
	latest = latest.RecordContribution(domain.GoalContribution{Amount: money.New(600, "USD")})
	if err := app.PutGoal(latest); err != nil {
		t.Fatal(err)
	}
	if err := app.PutAccount(domain.Account{ID: latest.AccountID, Name: "New bank", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"}); err != nil {
		t.Fatal(err)
	}
	result, err := app.ContributeToGoal(stale, money.New(500, "USD"), true)
	if err != nil {
		t.Fatal(err)
	}
	stored := findGoal(t, app, stale.ID)
	if stored.Name != latest.Name || stored.Notes != latest.Notes || stored.Priority != latest.Priority || stored.AccountID != latest.AccountID || stored.CurrentAmount.Amount != 1200 || len(stored.Contributions) != 2 {
		t.Fatalf("stale contribution overwrote current goal fields: %+v", stored)
	}
	if result.BeforeSavedPercent != 35 || result.AfterSavedPercent != 60 || result.Milestone != 50 || app.Transactions()[0].AccountID != latest.AccountID {
		t.Fatalf("stale progress or account determined contribution facts: %+v", result)
	}
	if _, ok, err := app.UndoLastContribution(stale); err != nil || !ok {
		t.Fatalf("stale undo failed: %v", err)
	}
	stored = findGoal(t, app, stale.ID)
	if stored.CurrentAmount.Amount != 700 || len(stored.Contributions) != 1 || stored.Name != latest.Name || stored.Notes != latest.Notes || hasTxn(app, result.TransactionID) {
		t.Fatal("stale undo overwrote metadata or reversed the wrong contribution")
	}
	if err := app.DeleteGoal(stale.ID); err != nil {
		t.Fatal(err)
	}
	revision := app.Rev()
	if _, err := app.ContributeToGoal(stale, money.New(100, "USD"), true); err == nil {
		t.Fatal("deleted goal was resurrected")
	}
	if _, _, err := app.UndoLastContribution(stale); err == nil {
		t.Fatal("undo resurrected a deleted goal")
	}
	if len(app.Goals()) != 0 || len(app.Transactions()) != 0 || app.Rev() != revision {
		t.Fatal("missing goal mutation changed store state")
	}
}

func TestContributeUsesCanonicalSavedMilestonesAndKnownCoverage(t *testing.T) {
	for _, test := range []struct {
		name                    string
		saved, reserved, amount int64
		rates                   map[string]float64
		complete, known         bool
		milestone               int
	}{
		{"foreign reservation completes coverage", 100, 400, 100, map[string]float64{"EUR": 2}, true, true, 0},
		{"already covered does not complete again", 100, 450, 100, map[string]float64{"EUR": 2}, false, true, 0},
		{"unknown reservations never prompt completion", 950, 2000, 50, nil, false, false, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			app := newApp(t, false)
			defer app.Store().Close()
			if err := app.PutSettings(store.Settings{BaseCurrency: "USD", FXRates: test.rates}); err != nil {
				t.Fatal(err)
			}
			goal := contributionSafetyGoal(test.saved)
			goal.Allocations = []domain.GoalAllocation{{AccountID: "foreign", Amount: money.New(test.reserved, "EUR")}}
			if err := app.PutGoal(goal); err != nil {
				t.Fatal(err)
			}
			result, err := app.ContributeToGoal(goal, money.New(test.amount, "USD"), false)
			if err != nil || result.BecameComplete != test.complete || result.CompletionKnown != test.known || !result.SavedProgressKnown || result.Milestone != test.milestone {
				t.Fatalf("incorrect completion or saved milestone facts: %+v %v", result, err)
			}
			if got := findGoal(t, app, goal.ID).Allocations; !reflect.DeepEqual(got, goal.Allocations) {
				t.Fatal("contribution rewrote foreign reservations")
			}
		})
	}
}

func TestUndoContributionRejectsUnsafeLogsWithoutMutation(t *testing.T) {
	for _, test := range []struct {
		name    string
		current int64
		logged  money.Money
	}{
		{"currency", 100, money.New(50, "EUR")},
		{"negative", 100, money.New(-1, "USD")},
		{"overflow", math.MinInt64, money.New(1, "USD")},
	} {
		t.Run(test.name, func(t *testing.T) {
			app := newApp(t, false)
			defer app.Store().Close()
			goal := contributionSafetyGoal(test.current).RecordContribution(domain.GoalContribution{Amount: test.logged, TxnID: "keep"})
			if err := app.Store().PutGoal(goal); err != nil {
				t.Fatal(err)
			}
			if err := app.Store().PutTransaction(domain.Transaction{ID: "keep", AccountID: "bank", Amount: money.New(-50, "USD")}); err != nil {
				t.Fatal(err)
			}
			revision := app.Rev()
			if _, ok, err := app.UndoLastContribution(goal); err == nil || ok {
				t.Fatal("unsafe undo succeeded")
			}
			if app.Rev() != revision || !reflect.DeepEqual(findGoal(t, app, goal.ID), goal) || !hasTxn(app, "keep") {
				t.Fatal("unsafe undo changed the goal/log/ledger")
			}
		})
	}
	app := newApp(t, false)
	defer app.Store().Close()
	goal := contributionSafetyGoal(10).RecordContribution(domain.GoalContribution{Amount: money.New(50, "USD"), TxnID: "already-removed"})
	if err := app.PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := app.UndoLastContribution(goal); err != nil || !ok || findGoal(t, app, goal.ID).CurrentAmount.Amount != 0 {
		t.Fatalf("safe floor-at-zero undo changed: %v", err)
	}
	app.SetActiveRoleFunc(func() domain.MemberRole { return domain.RoleViewer })
	if _, err := app.ContributeToGoal(goal, money.New(1, "USD"), true); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("read-only contribution guard changed: %v", err)
	}
}

func TestPutGoalTriggersOnlyKnownCanonicalCoverageTransition(t *testing.T) {
	app := newApp(t, false)
	defer app.Store().Close()
	if err := app.PutSettings(store.Settings{BaseCurrency: "USD", FXRates: map[string]float64{"EUR": 2}}); err != nil {
		t.Fatal(err)
	}
	if err := app.PutWorkflow(workflow.Workflow{ID: "reached", Name: "Reached", Enabled: true, Trigger: workflow.Trigger{Kind: workflow.TriggerGoalReached},
		Actions: []workflow.Action{{Kind: workflow.ActionCreateTask, Title: "Celebrate"}}}); err != nil {
		t.Fatal(err)
	}
	goal := contributionSafetyGoal(100)
	goal.Allocations = []domain.GoalAllocation{{AccountID: "foreign", Amount: money.New(400, "EUR")}}
	if err := app.PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	goal.CurrentAmount = money.New(200, "USD")
	if err := app.PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	if len(app.Tasks()) != 1 || len(app.WorkflowRuns()) != 1 {
		t.Fatal("converted coverage completion did not trigger exactly once")
	}
	goal.Notes = "Metadata only"
	if err := app.PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	if len(app.Tasks()) != 1 {
		t.Fatal("metadata save repeated completion trigger")
	}
	unknown := contributionSafetyGoal(950)
	unknown.ID = "unknown"
	unknown.Allocations = []domain.GoalAllocation{{AccountID: "missing", Amount: money.New(2000, "JPY")}}
	if err := app.PutGoal(unknown); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ContributeToGoal(unknown, money.New(50, "USD"), false); err != nil {
		t.Fatal(err)
	}
	if len(app.Tasks()) != 1 {
		t.Fatal("unknown foreign reservation triggered false completion")
	}
	legacy := contributionSafetyGoal(2000)
	legacy.ID, legacy.CurrentAmount = "legacy", money.New(2000, "GBP")
	if err := app.Store().PutGoal(legacy); err != nil {
		t.Fatal(err)
	}
	legacy.CurrentAmount = money.New(1000, "USD")
	if err := app.PutGoal(legacy); err != nil {
		t.Fatal(err)
	}
	if len(app.Tasks()) != 1 {
		t.Fatal("unknown prior saved coverage was treated as a known completion transition")
	}
}

func TestGoalContributionRemainingUsesCurrentGoalAndCheckedMoney(t *testing.T) {
	app := newApp(t, false)
	defer app.Store().Close()
	goal := contributionSafetyGoal(0)
	goal.CurrentAmount = money.Money{}
	if err := app.PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	if got, err := app.GoalContributionRemaining(goal.ID); err != nil || got != money.New(1000, "USD") {
		t.Fatalf("legacy zero quick-fill changed: %+v %v", got, err)
	}
	goal.CurrentAmount = money.New(900, "USD")
	if err := app.PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	if got, err := app.GoalContributionRemaining(goal.ID); err != nil || got.Amount != 100 {
		t.Fatalf("quick-fill ignored current saved amount: %+v %v", got, err)
	}
	goal.CurrentAmount = money.New(math.MinInt64, "USD")
	if err := app.Store().PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	if _, err := app.GoalContributionRemaining(goal.ID); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("remaining overflow was presented as usable money: %v", err)
	}
}
