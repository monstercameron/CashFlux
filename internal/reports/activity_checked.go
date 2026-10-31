// SPDX-License-Identifier: MIT

package reports

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

// ValidateActivityAmounts checks the monetary domain used by legacy grouping
// reports before they run: period totals plus split-attributed magnitudes must
// fit int64. It does not net refunds or change transaction/category assignment.
// Callers choose the relevant income/expense population before validation.
// splitIncome additionally validates the absolute split attribution used by
// IncomeByCategory; parent-only reports do not require split-line FX.
func ValidateActivityAmounts(all budgeting.NettedLedger, start, end time.Time, rates currency.Rates, splitIncome bool) error {
	if _, _, err := ledger.PeriodTotals(all, start, end, rates); err != nil {
		return err
	}
	var incomeParts, expenseParts int64
	for _, txn := range all {
		if !txn.CountsInReports() || txn.IsTransfer() || !dateutil.InRange(txn.Date, start, end) || (!txn.IsIncome() && !txn.IsExpense()) {
			continue
		}
		if _, err := txn.Amount.AbsChecked(); err != nil {
			return err
		}
		whole, err := rates.Convert(txn.Amount, rates.Base)
		if err != nil {
			return err
		}
		whole, err = whole.AbsChecked()
		if err != nil {
			return err
		}
		parts := whole.Amount
		if splitIncome && txn.IsIncome() && txn.HasSplits() {
			parts = 0
			for _, line := range txn.Splits {
				if txn.IsExpense() && line.Amount.Amount >= 0 {
					continue
				}
				if _, err := line.Amount.AbsChecked(); err != nil {
					return err
				}
				converted, err := rates.Convert(line.Amount, rates.Base)
				if err != nil {
					return err
				}
				magnitude, err := converted.AbsChecked()
				if err != nil {
					return err
				}
				parts, err = money.Add(parts, magnitude.Amount)
				if err != nil {
					return fmt.Errorf("transaction %q split magnitudes: %w", txn.ID, err)
				}
			}
			if txn.IsIncome() && parts < whole.Amount {
				parts = whole.Amount
			}
		}
		if txn.IsIncome() {
			incomeParts, err = money.Add(incomeParts, parts)
		} else {
			expenseParts, err = money.Add(expenseParts, parts)
		}
		if err != nil {
			return fmt.Errorf("report attributed amounts: %w", err)
		}
	}
	return nil
}
