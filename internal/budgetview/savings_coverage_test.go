// SPDX-License-Identifier: MIT

package budgetview

import (
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestSavingsAccountsUsesConvertedReservationsForRemaining(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	account := savingsTestAccount("savings", "alice")
	goal := savingsTestGoal("goal", account.ID, "alice", 1000, now.AddDate(0, 5, 0))
	goal.CurrentAmount = money.New(100, "USD")
	goal.Allocations = []domain.GoalAllocation{{AccountID: "other", Amount: money.New(400, "EUR")}}
	before := goal
	before.Allocations = append([]domain.GoalAllocation(nil), goal.Allocations...)
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
	row := SavingsAccounts([]domain.Account{account}, []domain.Goal{goal}, "alice", now, "USD", rates)[0]
	if row.GoalError != nil || row.GoalComplete || row.RateMonths != 1 || row.SyncMinor != 100 {
		t.Fatalf("converted reservation did not reduce the remaining amount to $1: %+v", row)
	}
	if !reflect.DeepEqual(goal, before) {
		t.Fatal("native projection rewrote a saved amount or foreign reservation")
	}
	goal.Allocations[0].Amount = money.New(450, "EUR")
	row = SavingsAccounts([]domain.Account{account}, []domain.Goal{goal}, "alice", now, "USD", rates)[0]
	if row.GoalError != nil || !row.GoalComplete || row.RateMonths != 0 {
		t.Fatalf("converted fully covered goal was not recognized: %+v", row)
	}
}

func TestSavingsAccountsUnknownCoverageIsNeverFullyFunded(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	account := savingsTestAccount("savings", "alice")
	for _, source := range []string{"saved", "reserved"} {
		t.Run(source, func(t *testing.T) {
			goal := savingsTestGoal("unknown", account.ID, "alice", 1000, now.AddDate(0, 1, 0))
			if source == "saved" {
				goal.CurrentAmount = money.New(2000, "EUR")
			} else {
				goal.Allocations = []domain.GoalAllocation{{AccountID: "other", Amount: money.New(2000, "EUR")}}
			}
			row := SavingsAccounts([]domain.Account{account}, []domain.Goal{goal}, "alice", now, "USD", currency.Rates{Base: "USD"})[0]
			if !row.HasGoal || row.GoalID != goal.ID || row.GoalError == nil || row.GoalComplete || row.RateMonths != 0 || row.SyncMinor != 0 || len(row.GoalUnavailableCurrencies) == 0 {
				t.Fatalf("unknown FX became completed, paceable or syncable: %+v", row)
			}
			if row.ConversionError != nil || row.AssignedBase != 100 {
				t.Fatal("unknown goal coverage corrupted the account's known monthly assignment")
			}
		})
	}
}
