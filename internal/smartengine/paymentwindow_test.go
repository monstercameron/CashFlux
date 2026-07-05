// SPDX-License-Identifier: MIT

package smartengine

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TestPaymentInWindowDayGranular pins the timezone trap SMART-BL3 fell into:
// transaction dates are UTC-midnight stamps while due dates are built in local
// time, so an instant comparison dropped a payment made ON the due date
// whenever the local zone was behind UTC — and the engine claimed a car
// payment "may have been missed" against a ledger that plainly held it.
func TestPaymentInWindowDayGranular(t *testing.T) {
	est := time.FixedZone("EST-ish", -4*60*60)
	// The June car payment, stamped at UTC midnight (how the seed and CSV
	// imports date rows).
	txns := []domain.Transaction{{
		ID: "tx", AccountID: "acct-checking", TransferAccountID: "acct-carloan",
		Date: time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC), Amount: money.New(-62000, "USD"),
	}}
	// The due date as bl3MissedBill builds it: local-zone midnight on the 15th
	// (= 04:00 UTC — AFTER the transaction's instant, same calendar day).
	due := time.Date(2026, 6, 15, 0, 0, 0, 0, est)
	now := time.Date(2026, 7, 5, 9, 0, 0, 0, est)

	if !paymentInWindow(txns, "acct-carloan", due, now) {
		t.Fatal("a payment dated ON the due date must count regardless of zone offset")
	}
	// Sanity: a payment before the window still doesn't count.
	if paymentInWindow(txns, "acct-carloan", due.AddDate(0, 0, 1), now) {
		t.Fatal("a payment the day BEFORE the window must not count")
	}
}
