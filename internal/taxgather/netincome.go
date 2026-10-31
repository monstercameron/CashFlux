// SPDX-License-Identifier: MIT

package taxgather

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
)

// NetIncomeInput supplies immutable household tax records and exact half-open
// bounds. Year labels the result; it does not change the requested window.
// Include full history so out-of-window refund counterparts remain available.
type NetIncomeInput struct {
	Transactions []domain.Transaction
	Categories   []domain.Category
	Links        []domain.TxnLink
	Year         int
	From, To     time.Time
	Rates        currency.Rates
}

// NetIncome is report-eligible income less user-marked deductible spending,
// expressed in Currency's minor units. It does not establish business-only
// income, legal deductibility, an annual projection or an amount of tax owed.
// A successful zero or negative NetMinor is a known result, not missing data.
type NetIncome struct {
	Year                                   int
	From, To                               time.Time
	Currency                               string
	IncomeMinor, DeductibleMinor, NetMinor int64
}

// NetIncomeWithLinks reuses the checked reporting and category-spending folds over
// one explicitly netted refund ledger. A failed source fold returns an error
// and an empty result; callers must not feed that empty result to an estimator.
// Complete folds are intentional: even a nondeductible expense with unavailable
// FX can prevent the period's tax preparation. No partial totals are asserted.
func NetIncomeWithLinks(in NetIncomeInput) (NetIncome, error) {
	if in.Year < 1 || in.Year > 9999 || in.From.IsZero() || in.To.IsZero() || !in.From.Before(in.To) {
		return NetIncome{}, fmt.Errorf("invalid tax net-income year or window")
	}
	base := strings.ToUpper(strings.TrimSpace(in.Rates.Base))
	if !currency.Valid(base) {
		return NetIncome{}, fmt.Errorf("tax net-income base currency %q is unsupported", in.Rates.Base)
	}
	in.Rates.Base = base
	if err := validateRefunds(in.Transactions, in.Links, in.From, in.To); err != nil {
		return NetIncome{}, fmt.Errorf("tax net-income refund evidence: %w", err)
	}
	all := budgeting.NewNettedLedger(in.Transactions, in.Links)
	flows, err := reports.IncomeExpenseSeriesFromLedger(all, []time.Time{in.From, in.To}, in.Rates)
	if err != nil {
		return NetIncome{}, fmt.Errorf("tax net-income report flow: %w", err)
	}
	selected := make(map[string]bool, len(in.Categories))
	for _, category := range in.Categories {
		selected[category.ID] = category.Deductible
	}
	spending, err := ledger.CategorySpendSeriesSelected(all, []time.Time{in.From, in.To}, in.Rates, selected)
	if err != nil {
		return NetIncome{}, fmt.Errorf("tax net-income deductions: %w", err)
	}
	var deductible int64
	for _, amounts := range spending {
		deductible, err = money.Add(deductible, amounts[0])
		if err != nil {
			return NetIncome{}, fmt.Errorf("tax net-income deductions: %w", err)
		}
	}
	net, err := money.Sub(flows[0].Income, deductible)
	if err != nil {
		return NetIncome{}, fmt.Errorf("tax net income: %w", err)
	}
	return NetIncome{Year: in.Year, From: in.From, To: in.To, Currency: base,
		IncomeMinor: flows[0].Income, DeductibleMinor: deductible, NetMinor: net}, nil
}
