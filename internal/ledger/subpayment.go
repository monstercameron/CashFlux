// SPDX-License-Identifier: MIT

package ledger

import (
	"math"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// SubscriptionPaymentInfo summarizes the transactions a user has marked as payments
// toward one subscription (keyed by name): the most recent payment's amount (its
// magnitude), which transaction that was, and how many linked payments exist.
type SubscriptionPaymentInfo struct {
	Latest    money.Money // amount of the most recent linked payment (magnitude)
	LatestTxn string      // its transaction id
	Count     int         // how many transactions are linked to this subscription
	HasAny    bool        // whether any payment is linked
}

// SubscriptionPaymentForName scans txns for those marked as payments toward the
// subscription named name (Transaction.SubscriptionName) and returns the most recent
// one — by date, then by later position on a tie — as the subscription's last
// confirmed payment, plus the total count. Amounts are magnitudes (Abs) so they read
// as "what was paid" regardless of the ledger sign. An empty name yields a zero result.
func SubscriptionPaymentForName(name string, txns []domain.Transaction) SubscriptionPaymentInfo {
	if name == "" {
		return SubscriptionPaymentInfo{}
	}
	var info SubscriptionPaymentInfo
	var latest time.Time
	for _, t := range txns {
		if t.SubscriptionName != name {
			continue
		}
		isFirst := !info.HasAny
		info.Count++
		info.HasAny = true
		if isFirst || !t.Date.Before(latest) {
			latest = t.Date
			info.LatestTxn = t.ID
			mag, err := t.Amount.AbsChecked()
			if err != nil {
				mag = money.New(math.MaxInt64, t.Amount.Currency)
			}
			info.Latest = mag
		}
	}
	return info
}
