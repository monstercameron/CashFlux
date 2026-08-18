// SPDX-License-Identifier: MIT

package provisional

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/reconcile"
)

// This file answers one question for reports: is this period FINISHED, or is it
// still being written (C693)?
//
// A month whose statements have all arrived and reconciled is settled — its
// figures will not move again, and they can be compared against other settled
// months. The month you are living in is not: it holds a few weeks of
// transactions, whatever has not posted yet, and any provisional balance
// checkpoint standing in for the gap. Printing the two side by side with the same
// styling invites the comparison that matters least — a month-to-date total
// always looks like a collapse in spending, because it is three weeks short.

// Status is how settled a period's figures are.
type Status int

const (
	// Closed means every account is reconciled through the end of the period. The
	// figures are final.
	Closed Status = iota
	// Provisional means the period runs past what has been reconciled. The figures
	// are real but incomplete, and will change.
	Provisional
)

// String renders the status for logs and test failures.
func (s Status) String() string {
	if s == Closed {
		return "closed"
	}
	return "provisional"
}

// ClosedThrough is the date the HOUSEHOLD's books are settled through: the
// earliest reconciled-through date across the accounts that participate.
//
// The earliest, not the latest. A month is only finished when every account that
// feeds it has confirmed its figures; one account reconciled to yesterday says
// nothing about a period the others have not closed, and taking the latest would
// let a single well-kept account certify months the rest have not.
//
// Accounts that are archived, or that are never reconciled against a statement at
// all (a utility shell, a property valuation), are skipped — waiting on a
// statement that will never arrive would leave the books permanently open.
//
// ok is false when no participating account has ever been reconciled, in which
// case nothing is closed and every period is provisional.
func ClosedThrough(accounts []domain.Account) (time.Time, bool) {
	var earliest time.Time
	found := false
	for _, a := range accounts {
		if a.Archived || !a.Type.IsReconcilable() {
			continue
		}
		through, ok := reconcile.Through(a.Reconciliations)
		if !ok {
			// An account that has never been reconciled leaves the books open, and
			// says so immediately: there is no earliest date to take.
			return time.Time{}, false
		}
		if !found || through.Before(earliest) {
			earliest, found = through, true
		}
	}
	return earliest, found
}

// StatusOf reports whether a period ending at end (exclusive, as every range in
// this app is) is closed given a household closed-through date.
//
// The comparison is against the period's last DAY rather than its exclusive end
// instant, so a month ending 1 August is closed by a statement reconciled through
// 31 July — which is what "closed through July" means to a person.
func StatusOf(end time.Time, closedThrough time.Time, hasClosed bool) Status {
	if !hasClosed {
		return Provisional
	}
	lastDay := end.AddDate(0, 0, -1)
	if !lastDay.After(dayEnd(closedThrough)) {
		return Closed
	}
	return Provisional
}

// Standing is what a report needs to caption a period honestly.
type Standing struct {
	// Status is closed or provisional.
	Status Status
	// CheckpointMinor is how much of the period's balances rests on provisional
	// checkpoints rather than on transactions. Reports exclude these rows, so this
	// is the figure that explains a balance and a period's activity disagreeing.
	CheckpointMinor int64
	// Checkpoints is how many such rows fall in the period.
	Checkpoints int
}

// StandingOf describes a period: whether it is closed, and how much of it is
// currently resting on a guess.
func StandingOf(txns []domain.Transaction, start, end time.Time, accounts []domain.Account) Standing {
	closedThrough, ok := ClosedThrough(accounts)
	st := Standing{Status: StatusOf(end, closedThrough, ok)}
	for _, t := range txns {
		if !t.IsBalanceCheckpoint() {
			continue
		}
		if t.Date.Before(start) || !t.Date.Before(end) {
			continue
		}
		st.Checkpoints++
		st.CheckpointMinor += t.Amount.Amount
	}
	return st
}

// RestsOnAGuess reports whether any of the period's balance comes from a
// checkpoint rather than from transactions — the thing worth saying out loud,
// because it is the difference between "you spent less" and "the month is not
// over".
func (s Standing) RestsOnAGuess() bool { return s.Checkpoints > 0 }
