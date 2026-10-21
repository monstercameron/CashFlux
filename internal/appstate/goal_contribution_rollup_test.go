// SPDX-License-Identifier: MIT

package appstate

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestContributePreflightsMonthlyHistoryBeforeLedgerPosting(t *testing.T) {
	const maxCount = int(^uint(0) >> 1)
	for _, test := range []struct {
		name        string
		monthAmount int64
		count       int
	}{
		{"amount overflow", math.MaxInt64, 1},
		{"count overflow", 100, maxCount},
	} {
		t.Run(test.name, func(t *testing.T) {
			app := newApp(t, false)
			defer app.Store().Close()
			if err := app.PutAccount(domain.Account{ID: "bank", Name: "Bank", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
				Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"}); err != nil {
				t.Fatal(err)
			}
			goal := contributionSafetyGoal(100)
			goal.ContributionHistory = []domain.GoalMonthTotal{{Month: "2026-07", Amount: money.New(test.monthAmount, "USD"), Count: test.count}}
			for i := 0; i < domain.MaxGoalContributions; i++ {
				goal.Contributions = append(goal.Contributions, domain.GoalContribution{Amount: money.New(1, "USD"), At: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)})
			}
			if err := app.PutGoal(goal); err != nil {
				t.Fatal(err)
			}
			before, revision := findGoal(t, app, goal.ID), app.Rev()
			if _, err := app.ContributeToGoal(goal, money.New(1, "USD"), true); err == nil || !strings.Contains(err.Error(), "record history") {
				t.Fatalf("monthly history failure was not reported: %v", err)
			}
			if app.Rev() != revision || !reflect.DeepEqual(findGoal(t, app, goal.ID), before) || len(app.Transactions()) != 0 {
				t.Fatal("history overflow posted a debit or changed saved progress/review/log")
			}
		})
	}
}

func TestContributeSuccessfulRolloverRetainsTxnLinkAndUndo(t *testing.T) {
	app := newApp(t, false)
	defer app.Store().Close()
	if err := app.PutAccount(domain.Account{ID: "bank", Name: "Bank", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"}); err != nil {
		t.Fatal(err)
	}
	goal := contributionSafetyGoal(100)
	goal.ContributionHistory = []domain.GoalMonthTotal{{Month: "2026-07", Amount: money.New(100, "USD"), Count: 1}}
	for i := 0; i < domain.MaxGoalContributions; i++ {
		goal.Contributions = append(goal.Contributions, domain.GoalContribution{Amount: money.New(1, "USD"), At: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)})
	}
	if err := app.PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	result, err := app.ContributeToGoal(goal, money.New(10, "USD"), true)
	if err != nil || result.TransactionID == "" {
		t.Fatalf("valid rollover contribution failed: %+v %v", result, err)
	}
	stored := findGoal(t, app, goal.ID)
	last := stored.Contributions[len(stored.Contributions)-1]
	if len(stored.Contributions) != domain.MaxGoalContributions || last.Amount != money.New(10, "USD") || last.TxnID != result.TransactionID || stored.ContributionHistory[0].Amount.Amount != 101 || stored.ContributionHistory[0].Count != 2 {
		t.Fatal("preflighted log lost its successful ledger link or aged monthly total")
	}
	if amount, ok, err := app.UndoLastContribution(stored); err != nil || !ok || amount.Amount != 10 || hasTxn(app, result.TransactionID) {
		t.Fatalf("rolled log's latest contribution was not undoable: %+v %v", amount, err)
	}
	if got := findGoal(t, app, goal.ID); got.CurrentAmount.Amount != 100 || got.ContributionHistory[0].Amount.Amount != 101 {
		t.Fatal("undo incorrectly reversed aged-out history")
	}
}
