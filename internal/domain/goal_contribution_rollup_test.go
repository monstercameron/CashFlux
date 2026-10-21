// SPDX-License-Identifier: MIT

package domain

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/money"
)

func rolloverGoal(aged GoalContribution, history []GoalMonthTotal) Goal {
	goal := Goal{ID: "rollover", Contributions: make([]GoalContribution, MaxGoalContributions), ContributionHistory: history}
	for i := range goal.Contributions {
		goal.Contributions[i] = GoalContribution{Amount: money.New(1, "USD"), At: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)}
	}
	goal.Contributions[0] = aged
	return goal
}

func copyRolloverGoal(goal Goal) Goal {
	goal.Contributions = append([]GoalContribution(nil), goal.Contributions...)
	goal.ContributionHistory = append([]GoalMonthTotal(nil), goal.ContributionHistory...)
	return goal
}

func TestRecordContributionCheckedRollupBoundariesAndNonmutation(t *testing.T) {
	const maxCount = int(^uint(0) >> 1)
	for _, test := range []struct {
		name                string
		initial, aged, want int64
	}{
		{"positive boundary", math.MaxInt64 - 1, 1, math.MaxInt64},
		{"negative boundary", math.MinInt64 + 1, -1, math.MinInt64},
	} {
		t.Run(test.name, func(t *testing.T) {
			aged := GoalContribution{Amount: money.New(test.aged, "USD"), At: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)}
			goal := rolloverGoal(aged, []GoalMonthTotal{{Month: "2026-07", Amount: money.New(test.initial, "USD"), Count: maxCount - 1}})
			before := copyRolloverGoal(goal)
			newEntry := GoalContribution{Amount: money.New(2, "USD"), TxnID: "new", At: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)}
			updated, err := goal.RecordContributionChecked(newEntry)
			if err != nil || len(updated.Contributions) != MaxGoalContributions || updated.Contributions[MaxGoalContributions-1] != newEntry || updated.ContributionHistory[0].Amount.Amount != test.want || updated.ContributionHistory[0].Count != maxCount {
				t.Fatalf("representable boundary changed: %+v %v", updated, err)
			}
			updated.Contributions[0].Amount.Amount++
			updated.ContributionHistory[0].Month = "changed"
			if !reflect.DeepEqual(goal, before) {
				t.Fatal("checked rollover mutated or shared writable slices with its input")
			}
		})
	}
}

func TestRecordContributionCheckedRollupFailuresReturnOriginal(t *testing.T) {
	const maxCount = int(^uint(0) >> 1)
	for _, test := range []struct {
		name          string
		initial, aged int64
		count         int
	}{
		{"positive overflow", math.MaxInt64, 1, 1},
		{"negative underflow", math.MinInt64, -1, 1},
		{"count overflow", 100, 1, maxCount},
	} {
		t.Run(test.name, func(t *testing.T) {
			aged := GoalContribution{Amount: money.New(test.aged, "USD"), At: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)}
			goal := rolloverGoal(aged, []GoalMonthTotal{{Month: "2026-07", Amount: money.New(test.initial, "USD"), Count: test.count}})
			before := copyRolloverGoal(goal)
			newEntry := GoalContribution{Amount: money.New(2, "USD"), At: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)}
			updated, err := goal.RecordContributionChecked(newEntry)
			if err == nil {
				t.Fatal("unrepresentable monthly history succeeded")
			}
			if test.name == "count overflow" {
				if !strings.Contains(err.Error(), "count overflow") {
					t.Fatalf("wrong count failure: %v", err)
				}
			} else if !errors.Is(err, money.ErrOverflow) {
				t.Fatalf("amount overflow lost its cause: %v", err)
			}
			if !reflect.DeepEqual(updated, before) || !reflect.DeepEqual(goal, before) {
				t.Fatal("failed rollup changed original or returned a partial log/history")
			}
			if legacy := goal.RecordContribution(newEntry); !reflect.DeepEqual(legacy, before) {
				t.Fatal("legacy recording persisted an overflowed monthly history")
			}
		})
	}
}

func TestRecordContributionCheckedMultipleAgedEntriesAreAllOrNothing(t *testing.T) {
	goal := rolloverGoal(GoalContribution{Amount: money.New(1, "USD"), At: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)},
		[]GoalMonthTotal{{Month: "2026-06", Amount: money.New(100, "USD"), Count: 1}, {Month: "2026-07", Amount: money.New(math.MaxInt64, "USD"), Count: 1}})
	goal.Contributions = append([]GoalContribution{{Amount: money.New(1, "USD"), At: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)}}, goal.Contributions...)
	// Put the representable June rollover first, then the failing July rollover.
	goal.Contributions[0], goal.Contributions[1] = goal.Contributions[1], goal.Contributions[0]
	before := copyRolloverGoal(goal)
	updated, err := goal.RecordContributionChecked(GoalContribution{Amount: money.New(2, "USD"), At: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)})
	if !errors.Is(err, money.ErrOverflow) || !reflect.DeepEqual(updated, before) || !reflect.DeepEqual(goal, before) {
		t.Fatal("a later failing rollover retained an earlier partial month update")
	}
}

func TestRecordContributionCheckedPreservesActualAgedCurrencyMismatchAndHistoryCap(t *testing.T) {
	aged := GoalContribution{Amount: money.New(50, "EUR"), At: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)}
	goal := rolloverGoal(aged, []GoalMonthTotal{{Month: "2026-07", Amount: money.New(100, "USD"), Count: 2}})
	entry := GoalContribution{Amount: money.New(2, "USD"), At: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)}
	updated, err := goal.RecordContributionChecked(entry)
	if err != nil || !reflect.DeepEqual(updated.ContributionHistory, goal.ContributionHistory) || updated.Contributions[0] == aged || len(updated.Contributions) != MaxGoalContributions {
		t.Fatal("existing mismatch-skipping policy changed or test failed to age out the foreign entry")
	}
	start := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	var history []GoalMonthTotal
	for i := range ContributionHistoryCap {
		history = append(history, GoalMonthTotal{Month: start.AddDate(0, i, 0).Format("2006-01"), Amount: money.New(1, "USD"), Count: 1})
	}
	aged.Amount = money.New(50, "USD")
	goal = rolloverGoal(aged, history)
	before := copyRolloverGoal(goal)
	updated, err = goal.RecordContributionChecked(entry)
	if err != nil || len(updated.ContributionHistory) != ContributionHistoryCap || updated.ContributionHistory[0].Month != "2000-02" || updated.ContributionHistory[ContributionHistoryCap-1].Month != "2026-07" || !reflect.DeepEqual(goal, before) {
		t.Fatal("history cap/sort/copy semantics changed")
	}
}
