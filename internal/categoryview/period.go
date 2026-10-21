// SPDX-License-Identifier: MIT

// Package categoryview prepares native facts for the category ledger page.
// It keeps reporting arithmetic and transaction scans out of screen rendering.
package categoryview

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
)

// PeriodFacts contains independently available transaction counts, expense
// rows, income rows, and their share percentages. Spending data is prepared
// from an explicit refund-netted ledger; income preserves IncomeByCategory's
// existing non-netted categorization policy. Errors keep an unavailable result
// distinct from a valid zero.
type PeriodFacts struct {
	TxnCountByCategory      map[string]int
	LifetimeUsageByCategory map[string]int

	SpendingAvailable       bool
	SpendingErr             error
	SpendByCategory         map[string]int64
	TotalSpend              int64
	TotalSpendAvailable     bool
	TotalSpendErr           error
	UnfiledSpend            int64
	MaxSpend                int64
	TopSpendCategoryID      string
	SpendSharePctByCategory map[string]int
	SpendSharesAvailable    bool
	SpendSharesErr          error

	IncomeAvailable          bool
	IncomeErr                error
	IncomeByCategory         map[string]int64
	MaxIncome                int64
	IncomeSharePctByCategory map[string]int
	IncomeSharesAvailable    bool
	IncomeSharesErr          error
}

// PeriodInput groups one render's immutable ledger and explicit refund links.
// A link-read failure only invalidates spending, because income aggregation does
// not apply refund netting under its existing report contract.
type PeriodInput struct {
	Transactions []domain.Transaction
	Budgets      []domain.Budget
	TxnLinks     []domain.TxnLink
	TxnLinksErr  error
}

// PreparePeriod scans immutable transaction input once for category usage and
// uses the canonical checked report folds for period amounts. Independent
// families remain available when another family fails on FX or arithmetic.
func PreparePeriod(input PeriodInput, start, end time.Time, rates currency.Rates) PeriodFacts {
	facts := PeriodFacts{
		TxnCountByCategory:      make(map[string]int),
		LifetimeUsageByCategory: make(map[string]int),
	}
	for _, txn := range input.Transactions {
		facts.TxnCountByCategory[txn.CategoryID]++
		facts.LifetimeUsageByCategory[txn.CategoryID]++
	}
	for _, budget := range input.Budgets {
		// Preserve the existing category-page lifetime prompt contract: a budget
		// references its primary CategoryID here. The deletion guard separately
		// counts the complete set of live category references.
		facts.LifetimeUsageByCategory[budget.CategoryID]++
	}

	if input.TxnLinksErr != nil {
		facts.SpendingErr = input.TxnLinksErr
	} else {
		netted := budgeting.NewNettedLedger(input.Transactions, input.TxnLinks)
		spendingRows, err := reports.SpendingByCategoryChecked(netted, start, end, rates)
		if err != nil {
			facts.SpendingErr = err
		} else {
			facts.SpendingAvailable = true
			facts.SpendByCategory = make(map[string]int64, len(spendingRows))
			for _, row := range spendingRows {
				facts.SpendByCategory[row.CategoryID] = row.Amount
				if row.CategoryID == "" {
					facts.UnfiledSpend = row.Amount
					continue
				}
				if row.Amount > facts.MaxSpend {
					facts.MaxSpend = row.Amount
					facts.TopSpendCategoryID = row.CategoryID
				}
			}
			facts.TotalSpend, facts.TotalSpendErr = reports.TotalChecked(spendingRows)
			facts.TotalSpendAvailable = facts.TotalSpendErr == nil
			facts.SpendSharePctByCategory, facts.SpendSharesErr = sharePercents(spendingRows, facts.MaxSpend)
			facts.SpendSharesAvailable = facts.SpendSharesErr == nil
		}
	}

	incomeRows, err := reports.IncomeByCategoryChecked(input.Transactions, start, end, rates)
	if err != nil {
		facts.IncomeErr = err
	} else {
		facts.IncomeAvailable = true
		facts.IncomeByCategory = make(map[string]int64, len(incomeRows))
		for _, row := range incomeRows {
			facts.IncomeByCategory[row.CategoryID] = row.Amount
			if row.CategoryID != "" && row.Amount > facts.MaxIncome {
				facts.MaxIncome = row.Amount
			}
		}
		facts.IncomeSharePctByCategory, facts.IncomeSharesErr = sharePercents(incomeRows, facts.MaxIncome)
		facts.IncomeSharesAvailable = facts.IncomeSharesErr == nil
	}
	return facts
}

func sharePercents(rows []reports.CategorySpend, max int64) (map[string]int, error) {
	shares := make(map[string]int, len(rows))
	if max <= 0 {
		return shares, nil
	}
	for _, row := range rows {
		if row.CategoryID == "" {
			continue
		}
		pct, err := money.MulDiv(row.Amount, 100, max, money.RoundTowardZero)
		if err != nil {
			return nil, err
		}
		shares[row.CategoryID] = int(pct)
	}
	return shares, nil
}
