// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/billmatch"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// billMatchPastDays / billMatchFutureDays bound the occurrence window the
// auto-matcher considers: a payment posts on or near its due date, so we look a
// little ahead (a bill charged a few days early) and well behind (recently due
// occurrences that just settled).
const (
	billMatchPastDays   = 45
	billMatchFutureDays = 10
)

// recurringOccurrences expands one recurring rule into its concrete due dates
// within [from, to] (inclusive), stepping the cadence. NextDue anchors the walk;
// the cadence is stepped backward to cover the past window and forward to the
// end. Iterations are capped so a stale NextDue can't loop unbounded.
func recurringOccurrences(r domain.Recurring, from, to time.Time) []time.Time {
	if r.Cadence == "" || r.NextDue.IsZero() {
		return nil
	}
	// Step the cadence back to an anchor at or before `from`, then emit forward.
	anchor := r.NextDue
	for i := 0; i < 1000 && anchor.After(from); i++ {
		back := stepCadenceBack(r.Cadence, anchor)
		if !back.Before(anchor) {
			break // no progress — avoid infinite loop
		}
		anchor = back
	}
	var out []time.Time
	cur := anchor
	for i := 0; i < 1000 && !cur.After(to); i++ {
		if !cur.Before(from) {
			out = append(out, cur)
		}
		next := r.Cadence.Next(cur)
		if !next.After(cur) {
			break
		}
		cur = next
	}
	return out
}

// stepCadenceBack returns the due date one cadence step before `from`, the
// inverse of RecurringCadence.Next for the fixed-interval cadences and an
// approximate month/quarter/year step-back otherwise.
func stepCadenceBack(c domain.RecurringCadence, from time.Time) time.Time {
	switch c {
	case domain.CadenceDaily:
		return from.AddDate(0, 0, -1)
	case domain.CadenceWeekly:
		return from.AddDate(0, 0, -7)
	case domain.CadenceBiweekly:
		return from.AddDate(0, 0, -14)
	case domain.CadenceSemimonthly:
		return from.AddDate(0, 0, -15)
	case domain.CadenceQuarterly:
		return from.AddDate(0, -3, 0)
	case domain.CadenceYearly:
		return from.AddDate(0, -1, 0)
	default: // monthly
		return from.AddDate(0, -1, 0)
	}
}

// billMatchOccurrences builds the matcher's occurrence set from every recurring
// EXPENSE rule (money-out) over the auto-match window, with payees resolved to
// their clean display names (TX1) so matching keys on merchant identity.
func (a *App) billMatchOccurrences(now time.Time) []billmatch.Occurrence {
	res := a.PayeeResolver()
	from := now.AddDate(0, 0, -billMatchPastDays)
	to := now.AddDate(0, 0, billMatchFutureDays)
	var out []billmatch.Occurrence
	for _, r := range a.Recurring() {
		if r.Amount.Amount >= 0 {
			continue // only outflows (bills) are matched
		}
		payee := res.Resolve(r.Label)
		for _, due := range recurringOccurrences(r, from, to) {
			out = append(out, billmatch.Occurrence{
				RecurringID: r.ID,
				DueDate:     due,
				Payee:       payee,
				CategoryID:  r.CategoryID,
				AmountMinor: absInt64(r.Amount.Amount),
				Currency:    r.Amount.Currency,
			})
		}
	}
	return out
}

// billMatchTxns builds the matcher's candidate transactions (expenses only, with
// resolved payees).
func (a *App) billMatchTxns() []billmatch.Txn {
	res := a.PayeeResolver()
	var out []billmatch.Txn
	for _, t := range a.Transactions() {
		if t.IsTransfer() || !t.Amount.IsNegative() {
			continue
		}
		payee := res.Resolve(txnPayeeOrDesc(t))
		out = append(out, billmatch.Txn{
			ID:          t.ID,
			Date:        t.Date,
			Payee:       payee,
			CategoryID:  t.CategoryID,
			AmountMinor: t.Amount.Amount,
			Currency:    t.Amount.Currency,
		})
	}
	return out
}

// billMatchLinksByOccurrence indexes existing bill-match links by their
// occurrence key (billmatch.Key), for both the already-matched guard and paid-
// state reads.
func (a *App) billMatchLinksByOccurrence() map[string]domain.TxnLink {
	out := map[string]domain.TxnLink{}
	for _, l := range a.TxnLinks() {
		if rid, due, ok := l.OccurrenceRef(); ok {
			out[billmatch.Key(rid, due)] = l
		}
	}
	return out
}

// autoMatchBills runs the bill matcher on the current data and persists a
// bill-match link for every UNAMBIGUOUS 1:1 occurrence↔transaction pair it finds
// (TX9). It is the auto-match hook: it runs after a transaction mutation, never
// on a timer. Ambiguous occurrences are left for a manual match. It is
// idempotent — occurrences already linked are skipped, so re-running creates
// nothing new.
func (a *App) autoMatchBills(now time.Time) {
	if a.triggersSuspended {
		return
	}
	existing := a.billMatchLinksByOccurrence()
	already := make(map[string]string, len(existing))
	for k, l := range existing {
		already[k] = l.Primary()
	}
	matches := billmatch.AutoMatches(a.billMatchOccurrences(now), a.billMatchTxns(), already)
	for _, m := range matches {
		link := domain.TxnLink{
			Kind:           domain.TxnLinkBillMatch,
			TxnIDs:         []string{m.TxnID},
			RecurringID:    m.RecurringID,
			OccurrenceDate: m.DueDate,
		}
		if err := a.PutTxnLink(link); err != nil {
			a.logErr("autoMatchBills", err)
			continue
		}
		a.log.Info("bill auto-matched", "recurring", m.RecurringID,
			"due", m.DueDate.Format("2006-01-02"), "txn", m.TxnID, "variance", m.VarianceMinor)
	}
}

// BillMatchForOccurrence returns the bill-match link that settles the given
// recurring occurrence, or ok=false when the occurrence is unpaid.
func (a *App) BillMatchForOccurrence(recurringID string, due time.Time) (domain.TxnLink, bool) {
	l, ok := a.billMatchLinksByOccurrence()[billmatch.Key(recurringID, due)]
	return l, ok
}

// BillMatchVariance returns the matched payment's variance for an occurrence
// (actual magnitude − expected magnitude, signed) and ok=false when the
// occurrence is unpaid or its matched transaction is gone. A positive variance
// means the bill ran over the expected amount.
func (a *App) BillMatchVariance(recurringID string, due time.Time, expectedMinor int64) (int64, bool) {
	l, ok := a.BillMatchForOccurrence(recurringID, due)
	if !ok {
		return 0, false
	}
	t, found, err := a.store.GetTransaction(l.Primary())
	if err != nil || !found {
		return 0, false
	}
	return absInt64(t.Amount.Amount) - absInt64(expectedMinor), true
}

// BillMatchForTxn returns the bill-match link a transaction settles (as the
// paying transaction), or ok=false. The transaction-row "Unlink bill" affordance
// uses this to find the link to delete.
func (a *App) BillMatchForTxn(txnID string) (domain.TxnLink, bool) {
	for _, l := range a.TxnLinks() {
		if l.Kind == domain.TxnLinkBillMatch && l.HasTxn(txnID) {
			return l, true
		}
	}
	return domain.TxnLink{}, false
}

// BillMatchPaidOccurrences returns the set of occurrence keys (billmatch.Key)
// that carry a durable bill-match link — the auto/manually matched occurrences,
// so consumers (the payday pre-flight, the missing-transaction detector) can
// treat them as settled without re-deriving the match.
func (a *App) BillMatchPaidOccurrences() map[string]bool {
	out := map[string]bool{}
	for k := range a.billMatchLinksByOccurrence() {
		out[k] = true
	}
	return out
}

// UnlinkBill deletes the bill-match link that the given transaction settles,
// releasing the occurrence back to unpaid (manual unmatch). It errors when the
// transaction is not the paying side of any bill-match link.
func (a *App) UnlinkBill(txnID string) error {
	l, ok := a.BillMatchForTxn(txnID)
	if !ok {
		return fmt.Errorf("appstate: transaction %q is not matched to a bill", strings.TrimSpace(txnID))
	}
	return a.DeleteTxnLink(l.ID)
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
