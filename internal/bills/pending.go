// SPDX-License-Identifier: MIT

package bills

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/billmatch"
)

// PendingInWindow returns the bills due in [now, end) that do NOT already have
// a matching posted transaction — the local-first "pending" set: charges the
// schedule says are coming that the ledger hasn't posted yet. A bill is
// suppressed when the billmatch engine finds any candidate transaction for it
// (same tolerance/date-window/identity rules that settle recurring items), so
// a payment made early never shows twice — once posted, once "upcoming" (the
// commercial pending-vs-posted matching seam, without inventing a bank feed).
//
// all is typically UpcomingAll(...) output; txns are the ledger's transactions
// projected into billmatch.Txn (resolved payee, signed minor amount).
// settledAccounts holds the account IDs that already received an EXPLICIT bill
// payment in the window (a transaction's BillAccountID, or a transfer into the
// liability) — an exact link beats the fuzzy matcher, so "Priya's Car Loan"
// never shows as upcoming beside its posted "Car payment (Priya)" whose name
// the fuzzy identity check can't bridge.
func PendingInWindow(all []Bill, txns []billmatch.Txn, settledAccounts map[string]bool, now, end time.Time) []Bill {
	var out []Bill
	for _, b := range all {
		if b.DueDate.Before(day(now)) || !b.DueDate.Before(end) {
			continue
		}
		if b.AccountID != "" && settledAccounts[b.AccountID] {
			continue // an explicitly linked payment already covers this bill
		}
		occ := billmatch.Occurrence{
			DueDate:     b.DueDate,
			Payee:       b.Name,
			AmountMinor: -b.Amount.Amount, // bills carry positive magnitudes; expenses post negative
			Currency:    b.Amount.Currency,
		}
		if len(billmatch.Candidates(occ, txns, nil)) > 0 {
			continue // already posted (or a near-certain match exists)
		}
		out = append(out, b)
	}
	return out
}

// day truncates t to its calendar date (bills compare by date, not clock time).
func day(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
