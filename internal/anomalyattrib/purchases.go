// SPDX-License-Identifier: MIT

package anomalyattrib

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// CategoryPurchases collects the current month's reported expenses for one
// category and marks purchases more than half above that merchant's average
// over the three preceding complete months. Split receipts contribute only
// their negative lines in this category, combined into one purchase per receipt.
// Unconvertible or unrepresentable values are omitted.
func CategoryPurchases(category string, txns []domain.Transaction, rates currency.Rates, now time.Time, unnamed string) []Purchase {
	start := dateutil.MonthStart(now)
	baseline := dateutil.AddMonths(start, -3)
	end := dateutil.AddMonths(start, 1)
	var current []Purchase
	type history struct {
		sum   int64
		count int64
	}
	prior := make(map[string]history)
	for _, txn := range txns {
		if !txn.IsExpense() || !txn.CountsInReports() || !dateutil.InRange(txn.Date, baseline, end) {
			continue
		}
		minor, ok := categoryExpense(txn, category, rates)
		if !ok || minor <= 0 {
			continue
		}
		payee := strings.TrimSpace(txn.Payee)
		if payee == "" {
			payee = strings.TrimSpace(txn.Desc)
		}
		if payee == "" {
			payee = unnamed
		}
		if dateutil.InRange(txn.Date, start, end) {
			current = append(current, Purchase{ID: txn.ID, Payee: payee, Minor: minor})
			continue
		}
		key := strings.ToLower(payee)
		value := prior[key]
		if sum, err := money.Add(value.sum, minor); err == nil {
			prior[key] = history{sum: sum, count: value.count + 1}
		}
	}
	for i, purchase := range current {
		if previous := prior[strings.ToLower(purchase.Payee)]; previous.count > 0 {
			usual := previous.sum / previous.count
			// Compare the increment to half the usual amount. Multiplying the
			// two money figures by 2 and 3 can overflow for valid int64 amounts.
			current[i].Larger = usual > 0 && purchase.Minor > usual && purchase.Minor-usual > usual/2
		}
	}
	return current
}

func categoryExpense(txn domain.Transaction, category string, rates currency.Rates) (int64, bool) {
	if !txn.HasSplits() {
		if txn.CategoryID != category {
			return 0, false
		}
		value, err := rates.Convert(txn.Amount, rates.Base)
		if err != nil {
			return 0, false
		}
		magnitude, err := value.AbsChecked()
		return magnitude.Amount, err == nil
	}
	var total int64
	for _, split := range txn.Splits {
		if split.CategoryID != category || split.Amount.Amount >= 0 {
			continue
		}
		value, err := rates.Convert(split.Amount, rates.Base)
		if err != nil {
			return 0, false
		}
		magnitude, err := value.AbsChecked()
		if err != nil {
			return 0, false
		}
		total, err = money.Add(total, magnitude.Amount)
		if err != nil {
			return 0, false
		}
	}
	return total, true
}
