// SPDX-License-Identifier: MIT

package notificationview

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// NoSpendStreak counts completed days without an expense, ending yesterday and
// bounded by maximum. Today and transfers do not interrupt the streak. Dates
// retain the transaction's own calendar day, matching the ledger's date labels.
func NoSpendStreak(txns []domain.Transaction, now time.Time, maximum int) int {
	spendDays := make(map[string]bool)
	for _, txn := range txns {
		if txn.IsExpense() {
			spendDays[txn.Date.Format("2006-01-02")] = true
		}
	}
	for i := 1; i <= maximum; i++ {
		if spendDays[now.AddDate(0, 0, -i).Format("2006-01-02")] {
			return i - 1
		}
	}
	if maximum < 0 {
		return 0
	}
	return maximum
}
