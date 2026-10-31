// SPDX-License-Identifier: MIT

// Package periodview prepares checked period activity for Dashboard and Assistant.
package periodview

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Totals distinguishes a real zero-activity period from unavailable totals.
// Income, Expense and Net are usable only when Known is true. SavingsError can
// independently invalidate a percentage even when the money totals are valid.
type Totals struct {
	Known                bool
	Income, Expense, Net money.Money
	SavingsPercent       int64
	Error, SavingsError  error
}

// SavingsKnown reports whether the displayed whole-percent rate is available.
func (t Totals) SavingsKnown() bool { return t.Known && t.SavingsError == nil }

// BuildTotals preserves the ledger's all-or-nothing FX/arithmetic contract and
// derives net cash flow and a truncating savings percentage with checked math.
func BuildTotals(transactions []domain.Transaction, start, end time.Time, rates currency.Rates) Totals {
	income, expense, err := ledger.PeriodTotals(transactions, start, end, rates)
	if err != nil {
		return Totals{Error: err}
	}
	net, err := income.Sub(expense)
	if err != nil {
		return Totals{Error: err}
	}
	result := Totals{Known: true, Income: income, Expense: expense, Net: net}
	// Preserve the existing zero-income display convention; no division is made.
	if income.Amount > 0 {
		result.SavingsPercent, result.SavingsError = money.MulDiv(net.Amount, 100, income.Amount, money.RoundTowardZero)
	}
	return result
}
