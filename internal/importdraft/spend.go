// SPDX-License-Identifier: MIT

package importdraft

import (
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/spendsummary"
)

// SpendSummary retains the monthly preview or a checked arithmetic/currency error.
type SpendSummary struct {
	Months []spendsummary.MonthSpend
	Err    error
}

// MonthlySpend preserves the established date and amount parsing rules, and
// verifies that each monthly accumulator can be represented before publishing it.
// An explicitly different currency requires FX and cannot be silently aggregated.
func MonthlySpend(rows []extract.Row, cur string) SpendSummary {
	decimals := currency.Decimals(cur)
	checked := map[string]spendsummary.MonthSpend{}
	for _, row := range rows {
		if row.Currency != "" && !strings.EqualFold(row.Currency, cur) {
			return SpendSummary{Err: fmt.Errorf("summarize import spending: %w", money.ErrCurrencyMismatch)}
		}
		// One-row summaries preserve the existing tolerant month/date parser.
		month := spendsummary.Summarize([]extract.Row{row}, decimals)[0]
		if month.In < 0 || month.Out < 0 {
			return SpendSummary{Err: fmt.Errorf("summarize import magnitude: %w", money.ErrOverflow)}
		}
		total := checked[month.Month]
		var err error
		total.In, err = money.Add(total.In, month.In)
		if err != nil {
			return SpendSummary{Err: fmt.Errorf("summarize import income: %w", err)}
		}
		total.Out, err = money.Add(total.Out, month.Out)
		if err != nil {
			return SpendSummary{Err: fmt.Errorf("summarize import expenses: %w", err)}
		}
		checked[month.Month] = total
	}
	return SpendSummary{Months: spendsummary.Summarize(rows, decimals)}
}
