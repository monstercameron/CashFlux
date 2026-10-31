// SPDX-License-Identifier: MIT

package subscriptions

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

const trailingSpendMonthCount = 3

// TrailingSpend contains the pooled expenses and number of nonempty months used
// by the subscription spending-share denominator. Amounts are in the rate
// table's base-currency minor units.
type TrailingSpend struct {
	TotalExpense   int64
	EligibleMonths int
}

// TrailingSpendChecked totals expenses across the three completed calendar
// months before now. The current partial month is excluded, and months without
// expense do not contribute to EligibleMonths. sourceErr must carry any failure
// from reading the transaction snapshot; a partial snapshot is never accepted.
func TrailingSpendChecked(transactions []domain.Transaction, sourceErr error, now time.Time, rates currency.Rates) (TrailingSpend, error) {
	if sourceErr != nil {
		return TrailingSpend{}, fmt.Errorf("subscription trailing spend: %w", sourceErr)
	}

	var out TrailingSpend
	monthAnchor, _ := dateutil.MonthRange(now)
	for offset := 1; offset <= trailingSpendMonthCount; offset++ {
		// Subtract from the first of the current month so dates like May 31 or
		// March 31 cannot normalize into the current month or skip a month.
		start, end := dateutil.MonthRange(monthAnchor.AddDate(0, -offset, 0))
		_, expense, err := ledger.PeriodTotals(transactions, start, end, rates)
		if err != nil {
			return TrailingSpend{}, fmt.Errorf("subscription trailing spend for %s: %w", start.Format("2006-01"), err)
		}
		if expense.Amount == 0 {
			continue
		}
		out.TotalExpense, err = money.Add(out.TotalExpense, expense.Amount)
		if err != nil {
			return TrailingSpend{}, fmt.Errorf("subscription trailing spend total: %w", err)
		}
		out.EligibleMonths++
	}
	return out, nil
}
