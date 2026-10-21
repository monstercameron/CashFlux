// SPDX-License-Identifier: MIT

package notificationview

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/milestones"
	"github.com/monstercameron/CashFlux/internal/networthview"
)

// WinsInput supplies the household data used to detect financial milestones.
// Transactions are the raw ledger for net worth and no-spend days; BudgetLedger
// is the separately netted ledger so linked refunds retain budget semantics.
type WinsInput struct {
	Accounts     []domain.Account
	Transactions []domain.Transaction
	Tasks        []domain.Task
	Goals        []domain.Goal
	Budgets      []domain.Budget
	BudgetLedger budgeting.NettedLedger
	Rates        currency.Rates
	Now          time.Time
	WeekStart    time.Weekday
	NoSpendLimit int
	// NetWorth optionally reuses an already-computed availability-aware snapshot.
	// Nil computes it here. Partial or unavailable totals cannot establish a win.
	NetWorth *networthview.Snapshot
}

// PrepareWins computes the primitives consumed by milestones.Detect. It keeps
// goal order, counts only monthly budgets with positive spend at or below their
// limit in the previous completed month, and skips individual budget errors.
// Goal completion uses the canonical rate-aware evaluation; an unknown coverage
// result never becomes a celebrated win. No input data is modified; localization
// and seen-state stay with the caller.
func PrepareWins(in WinsInput) milestones.Input {
	var reached []string
	taskIndex := goals.BuildTaskIndex(in.Tasks)
	for _, goal := range in.Goals {
		evaluation := goals.EvaluateGoal(goal, taskIndex[goal.ID], in.Rates, in.Now)
		if evaluation.Err == nil && evaluation.Complete {
			reached = append(reached, goal.Name)
		}
	}
	var netWorth int64
	view := in.NetWorth
	if view == nil {
		computed := networthview.Build(in.Accounts, in.Transactions, in.Rates)
		view = &computed
	}
	if view.Complete() {
		netWorth = view.Net.Amount
	}
	lastMonth := in.Now.AddDate(0, 0, -in.Now.Day())
	start, end := budgeting.PeriodRange(domain.PeriodMonthly, lastMonth, in.WeekStart)
	kept := keptMonthlyBudgetWins(in.Budgets, in.BudgetLedger, start, end, in.Rates)
	return milestones.Input{
		ReachedGoals: reached, NetWorthMinor: netWorth,
		NoSpendDays: NoSpendStreak(in.Transactions, in.Now, in.NoSpendLimit),
		KeptBudgets: kept, KeptPeriodKey: lastMonth.Format("2006-01"), Now: in.Now,
	}
}
