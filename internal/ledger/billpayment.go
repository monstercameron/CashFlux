// SPDX-License-Identifier: MIT

package ledger

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// BillPaymentInfo summarizes the transactions a user has marked as bill payments
// toward one liability account: the most recent payment's amount (its magnitude),
// which transaction that was, and how many linked payments exist.
type BillPaymentInfo struct {
	Latest    money.Money // amount of the most recent linked payment (magnitude)
	LatestTxn string      // its transaction id
	Count     int         // how many transactions are linked to this account
	HasAny    bool        // whether any payment is linked
}

// BillPaymentForAccount scans txns for those marked as bill payments toward
// accountID (Transaction.BillAccountID) and returns the most recent one — by date,
// then by later position on a tie — as the account's actual monthly payment, plus
// the total count. Amounts are magnitudes (Abs) so they read as "what was paid"
// regardless of how the ledger signs the entry. accountID "" yields a zero result.
func BillPaymentForAccount(accountID string, txns []domain.Transaction) BillPaymentInfo {
	if accountID == "" {
		return BillPaymentInfo{}
	}
	var info BillPaymentInfo
	var latest time.Time
	for _, t := range txns {
		if t.BillAccountID != accountID {
			continue
		}
		info.Count++
		info.HasAny = true
		if info.LatestTxn == "" || !t.Date.Before(latest) {
			latest = t.Date
			info.LatestTxn = t.ID
			info.Latest = t.Amount.Abs()
		}
	}
	return info
}
