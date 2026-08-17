// SPDX-License-Identifier: MIT

// Package cancelwatch checks whether a cancelled subscription actually stopped
// charging (WF12).
//
// Cancelling is the easy half. The half that costs people money is finding out,
// four months later, that the charge never stopped — and the app already holds
// the only evidence that could tell them: the transactions that arrived after
// the cancellation date.
//
// Everything here is local arithmetic over the ledger. It never claims to have
// contacted anyone, and it never says a subscription IS cancelled — only what
// the charges since have done, which is the part it can actually see.
package cancelwatch

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// GraceDays is how long after a cancellation a charge is still expected.
//
// Thirty-five days: one monthly cycle plus a few, because the last charge often
// lands after the cancellation and billing dates drift. Flagging that charge as
// "still charging" would cry wolf on the single most common outcome, and a
// warning that is usually wrong is one people learn to dismiss.
const GraceDays = 35

// SettleDays is how long must pass with no charge before silence means stopped.
//
// Sixty-five days: two monthly cycles. One quiet cycle can be a billing date
// that moved; two is a pattern. Saying "stopped" after three weeks would be a
// confident answer to a question the calendar has not answered yet.
const SettleDays = 65

// Verdict is what the charges since a cancellation show.
type Verdict string

const (
	// VerdictTooSoon means not enough time has passed to say anything.
	//
	// A real answer, and the one this package exists to keep separate from
	// "stopped". Declaring success the day after somebody cancels is how a
	// monitor becomes reassurance.
	VerdictTooSoon Verdict = "tooSoon"
	// VerdictFinalCharge means one charge landed inside the grace window, which
	// is the expected ending rather than a problem.
	VerdictFinalCharge Verdict = "finalCharge"
	// VerdictStopped means nothing has arrived for long enough to believe it.
	VerdictStopped Verdict = "stopped"
	// VerdictStillCharging means money is still going out after the grace window.
	VerdictStillCharging Verdict = "stillCharging"
)

// Status is one cancelled subscription's standing.
type Status struct {
	CancellationID string
	Name           string
	CancelledOn    time.Time
	Verdict        Verdict
	// ChargesSince is how many charges arrived after the cancellation, and
	// TotalSinceMinor what they came to — the figure that turns "still charging"
	// from an annoyance into a number worth acting on.
	ChargesSince    int
	TotalSinceMinor int64
	// LastChargeOn is the most recent one, so a surface can say "still charging,
	// most recently on the 3rd" rather than leaving the reader to go looking.
	LastChargeOn time.Time
	// DaysSinceCancel is how long the ledger has had to show something.
	DaysSinceCancel int
}

// Acting reports whether this needs the household to do something.
func (s Status) Acting() bool { return s.Verdict == VerdictStillCharging }

// Check assesses every cancellation against the ledger.
//
// now is the clock, so a caller can ask "what did this look like last month"
// without the answer depending on when the function happens to run.
func Check(cancellations []domain.SubscriptionCancellation, txns []domain.Transaction, now time.Time) []Status {
	out := make([]Status, 0, len(cancellations))
	for _, c := range cancellations {
		if strings.TrimSpace(c.SubName) == "" || c.CancelledOn.IsZero() {
			// A cancellation with no name or no date cannot be matched against
			// anything. Skipped rather than reported as "stopped", which would be an
			// unearned reassurance about a record nobody can check.
			continue
		}
		s := Status{
			CancellationID: c.ID, Name: c.SubName, CancelledOn: c.CancelledOn,
			DaysSinceCancel: daysBetween(c.CancelledOn, now),
		}
		for _, t := range matching(txns, c.SubName) {
			if !t.Date.After(c.CancelledOn) {
				continue
			}
			if t.Date.After(now) {
				// A future-dated transaction has not happened yet, and counting it
				// would report a charge that has not arrived.
				continue
			}
			s.ChargesSince++
			amt := t.Amount.Amount
			if amt < 0 {
				amt = -amt
			}
			s.TotalSinceMinor += amt
			if t.Date.After(s.LastChargeOn) {
				s.LastChargeOn = t.Date
			}
		}
		s.Verdict = verdictFor(s)
		out = append(out, s)
	}
	// Newest cancellation first, ties by name, so the list is stable and the
	// recent decisions — the ones somebody is still watching — read first.
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].CancelledOn.Equal(out[j].CancelledOn) {
			return out[i].CancelledOn.After(out[j].CancelledOn)
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// verdictFor decides what the charges since add up to.
func verdictFor(s Status) Verdict {
	sinceLast := s.DaysSinceCancel
	if !s.LastChargeOn.IsZero() {
		sinceLast = daysBetween(s.LastChargeOn, s.CancelledOn.AddDate(0, 0, s.DaysSinceCancel))
	}
	switch {
	case s.ChargesSince == 0 && s.DaysSinceCancel >= SettleDays:
		return VerdictStopped
	case s.ChargesSince == 0:
		return VerdictTooSoon
	case s.ChargesSince == 1 && daysBetween(s.CancelledOn, s.LastChargeOn) <= GraceDays:
		// The expected ending: one last charge for the period already used.
		if sinceLast >= SettleDays {
			return VerdictStopped
		}
		return VerdictFinalCharge
	default:
		return VerdictStillCharging
	}
}

// matching returns transactions whose payee or description names the
// subscription.
//
// Substring, case-insensitive, on the household's own text — not fuzzy matching.
// A fuzzy match that quietly pulls in a different merchant would produce the
// worst possible output here: telling somebody a cancellation failed when it did
// not, which sends them to argue with a company that already did what they asked.
func matching(txns []domain.Transaction, name string) []domain.Transaction {
	needle := strings.ToLower(strings.TrimSpace(name))
	if needle == "" {
		return nil
	}
	var out []domain.Transaction
	for _, t := range txns {
		if !t.IsExpense() || !t.CountsInReports() {
			continue
		}
		if strings.Contains(strings.ToLower(t.Payee), needle) ||
			strings.Contains(strings.ToLower(t.Desc), needle) ||
			strings.EqualFold(strings.TrimSpace(t.SubscriptionName), strings.TrimSpace(name)) {
			out = append(out, t)
		}
	}
	return out
}

// daysBetween is whole days from a to b, negative when b precedes a.
func daysBetween(a, b time.Time) int {
	return int(b.Sub(a).Hours() / 24)
}

// StillCharging filters a checked list down to the ones needing action.
func StillCharging(all []Status) []Status {
	out := make([]Status, 0, len(all))
	for _, s := range all {
		if s.Acting() {
			out = append(out, s)
		}
	}
	return out
}
