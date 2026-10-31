// SPDX-License-Identifier: MIT

package budgetview

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func savingsTestAccount(id, owner string) domain.Account {
	return domain.Account{ID: id, Name: id, OwnerID: owner, Type: domain.TypeSavings, Currency: "USD", MonthlySavings: money.New(100, "USD")}
}

func savingsTestGoal(id, account, owner string, target int64, date time.Time) domain.Goal {
	return domain.Goal{ID: id, Name: id, AccountID: account, OwnerID: owner, TargetAmount: money.New(target, "USD"), CurrentAmount: money.Zero("USD"), TargetDate: date}
}

func TestSavingsAccountsPreservesVisibilityAndGoalSelection(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	accounts := []domain.Account{savingsTestAccount("shared", domain.GroupOwnerID), savingsTestAccount("mine", "alice"), savingsTestAccount("other", "bob"), savingsTestAccount("archived", "alice"), savingsTestAccount("checking", "alice")}
	accounts[3].Archived, accounts[4].Type = true, domain.TypeChecking
	goals := []domain.Goal{
		savingsTestGoal("later", "mine", "alice", 1000, now.AddDate(0, 5, 0)),
		savingsTestGoal("nearest", "mine", "alice", 500, now.AddDate(0, 2, 0)),
		savingsTestGoal("private", "mine", "bob", 500, now.AddDate(0, 0, 1)),
		savingsTestGoal("archived", "mine", "alice", 500, now.AddDate(0, 0, 1)),
		savingsTestGoal("fund", "mine", "alice", 500, now.AddDate(0, 0, 1)),
		savingsTestGoal("complete", "shared", domain.GroupOwnerID, 100, now),
		savingsTestGoal("habit", "mine", "alice", 500, now.AddDate(0, 0, 1)),
	}
	goals[3].Archived, goals[4].IsSinkingFund = true, true
	goals[5].CurrentAmount = money.New(100, "USD")
	goals[6].Kind = domain.GoalKindHabit
	goals[1].MonthlyContribution = money.New(100, "USD")
	beforeAccounts, beforeGoals := append([]domain.Account(nil), accounts...), append([]domain.Goal(nil), goals...)
	got := SavingsAccounts(accounts, goals, "alice", now, "USD", currency.Rates{Base: "USD"})
	if len(got) != 2 || got[0].AccountID != "mine" || got[1].AccountID != "shared" {
		t.Fatalf("member-visible sorted savings = %+v", got)
	}
	if row := got[0]; row.GoalID != "nearest" || row.MoreGoals != 1 || row.PlannedMonths != 2 || row.RateMonths != 5 || row.DeltaMonths != 3 || !row.Synced || row.SyncMinor != 100 || row.Type != "savings" {
		t.Fatalf("nearest incomplete goal projection = %+v", row)
	}
	if !got[1].HasGoal || !got[1].GoalComplete || got[1].RateMonths != 0 {
		t.Fatalf("fully funded goal disappeared: %+v", got[1])
	}
	if !reflect.DeepEqual(accounts, beforeAccounts) || !reflect.DeepEqual(goals, beforeGoals) {
		t.Fatal("derivation mutated account or goal snapshots")
	}
	if len(SavingsAccounts(accounts, goals, "", now, "USD", currency.Rates{Base: "USD"})) != 3 {
		t.Fatal("everyone scope omitted a private account")
	}
}

func TestSavingsAccountsConvertsMonthlyAssignmentsAndGoalRate(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	account := savingsTestAccount("foreign", "alice")
	account.Currency, account.MonthlySavings = "EUR", money.New(100, "")
	goal := savingsTestGoal("goal", "foreign", "alice", 1000, now.AddDate(0, 5, 0))
	got := SavingsAccounts([]domain.Account{account}, []domain.Goal{goal}, "alice", now, "USD", currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}})[0]
	if got.AssignedBase != 200 || got.Monthly != 100 || got.Currency != "EUR" || got.SyncMinor != 200 || got.SyncCurrency != "USD" || got.RateMonths != 5 || got.ConversionError != nil {
		t.Fatalf("cross-currency savings facts = %+v", got)
	}
	missing := SavingsAccounts([]domain.Account{account}, []domain.Goal{goal}, "alice", now, "USD", currency.Rates{Base: "USD"})[0]
	if missing.ConversionError == nil || missing.AssignedBase != 0 || missing.RateMonths != 0 || missing.SyncMinor != 0 {
		t.Fatalf("missing FX invented assignment or synchronization: %+v", missing)
	}
	account.MonthlySavings = money.Zero("")
	zero := SavingsAccounts([]domain.Account{account}, nil, "alice", now, "USD", currency.Rates{Base: "USD"})[0]
	if zero.ConversionError != nil || zero.AssignedBase != 0 {
		t.Fatal("zero assignment incorrectly requires an FX rate")
	}
}

func TestSavingsAccountsHugeProjectionDoesNotOverflowOrNarrow(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	account := savingsTestAccount("large", "alice")
	account.MonthlySavings = money.New(2, "USD")
	goal := savingsTestGoal("large", "large", "alice", math.MaxInt64, now.AddDate(0, 1, 0))
	got := SavingsAccounts([]domain.Account{account}, []domain.Goal{goal}, "alice", now, "USD", currency.Rates{Base: "USD"})[0]
	if got.RateMonths != math.MaxInt64/2+1 || got.DeltaMonths != math.MaxInt64/2 || got.SyncMinor != 2 {
		t.Fatalf("huge projection wrapped or narrowed: %+v", got)
	}
}

func TestSavingsMonthsUntilKeepsCalendarRounding(t *testing.T) {
	from := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		target time.Time
		want   int
	}{
		{time.Time{}, 0}, {from, 0}, {from.AddDate(0, 0, -1), 0},
		{time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), 1},
		{time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC), 2},
		{time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC), 12},
	} {
		if got := MonthsUntil(from, test.target); got != test.want {
			t.Errorf("MonthsUntil(%v, %v) = %d, want %d", from, test.target, got, test.want)
		}
	}
}
