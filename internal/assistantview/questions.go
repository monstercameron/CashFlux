// SPDX-License-Identifier: MIT

// Package assistantview derives the Assistant's display facts without UI hooks
// or browser dependencies. Callers own scope selection and money formatting.
package assistantview

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// QuestionFacts describes the data behind the Ask surface's starter questions.
// LargestExpense is retained so the caller can apply its usual payee labels.
type QuestionFacts struct {
	TopCategory         string
	NearLimitBudget     string
	UpcomingGoal        string
	OverBudgetCount     int
	UncategorizedCount  int
	LargestExpense      domain.Transaction
	LargestExpenseMinor int64
}

// Questions scans current-period expenses once for both category totals and the
// largest purchase. Category and budget ties preserve the supplied view order.
// Unconvertible purchases can still need categorization, but their raw foreign
// minor units must never become a base-currency amount in a question.
func Questions(txns []domain.Transaction, categories []domain.Category, statuses []budgeting.Status, goals []domain.Goal, rates currency.Rates, start, end, now time.Time) QuestionFacts {
	var out QuestionFacts
	topSpend := make(map[string]int64)
	for _, txn := range txns {
		if !txn.IsExpense() || txn.IsTransfer() || !txn.CountsInReports() || !dateutil.InRange(txn.Date, start, end) {
			continue
		}
		if hasUncategorizedExpense(txn) {
			out.UncategorizedCount++
		}
		amount, err := txn.Amount.AbsChecked()
		if err != nil {
			continue
		}
		converted, err := rates.Convert(amount, rates.Base)
		if err != nil {
			continue
		}
		if converted.Amount > out.LargestExpenseMinor {
			out.LargestExpense, out.LargestExpenseMinor = txn, converted.Amount
		}
		if !txn.HasSplits() {
			addSpend(topSpend, txn.CategoryID, converted.Amount)
			continue
		}
		for _, split := range txn.Splits {
			if split.Amount.Amount >= 0 {
				continue
			}
			magnitude, err := split.Amount.AbsChecked()
			if err != nil {
				continue
			}
			value, err := rates.Convert(magnitude, rates.Base)
			if err != nil {
				continue
			}
			addSpend(topSpend, split.CategoryID, value.Amount)
		}
	}
	var top int64
	for _, category := range categories {
		if value := topSpend[category.ID]; value > top {
			top, out.TopCategory = value, category.Name
		}
	}
	for _, status := range statuses {
		if out.NearLimitBudget == "" && (status.State == budgeting.StateNear || status.State == budgeting.StateOver) {
			out.NearLimitBudget = status.Budget.Name
		}
		if status.State == budgeting.StateOver {
			out.OverBudgetCount++
		}
	}
	var soonest time.Time
	for _, goal := range goals {
		if goal.Archived || goal.TargetDate.IsZero() || !goal.TargetDate.After(now) {
			continue
		}
		if soonest.IsZero() || goal.TargetDate.Before(soonest) {
			soonest, out.UpcomingGoal = goal.TargetDate, goal.Name
		}
	}
	return out
}

func hasUncategorizedExpense(txn domain.Transaction) bool {
	if !txn.HasSplits() {
		return txn.CategoryID == ""
	}
	for _, split := range txn.Splits {
		if split.CategoryID == "" && split.Amount.Amount < 0 {
			return true
		}
	}
	return false
}

func addSpend(totals map[string]int64, category string, value int64) {
	if sum, err := money.Add(totals[category], value); err == nil {
		totals[category] = sum
	}
}
