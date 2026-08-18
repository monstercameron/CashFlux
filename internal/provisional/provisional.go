// SPDX-License-Identifier: MIT

// Package provisional owns balance checkpoints: the adjustments written when
// someone updates an account's balance from a figure they read somewhere, before
// the statement that will confirm it arrives (C684).
//
// # Why they need their own handling
//
// A checkpoint is real for balances and fiction for reporting. The account really
// does hold that much — that is the whole point of entering it — but the
// difference was not earned or spent, so counting it as income or spending
// invents a payday or a shopping trip that never happened. Before this, updating
// a balance wrote an ordinary "Balance adjustment" transaction, and getting that
// right meant the user remembering to tag it and tick "exclude from reports" by
// hand, every time.
//
// # The part that was missing entirely
//
// Nothing ever took one away. When the statement finally landed and its rows were
// imported, the checkpoint stayed — so the account carried both the real
// transactions AND the adjustment that had been standing in for them, and drifted
// by exactly the adjustment. A stand-in that is never removed is worse than no
// stand-in, because the error compounds with every month someone checks their
// balance.
//
// So a checkpoint is superseded two ways: a newer checkpoint on the same account
// replaces it (there is only ever one open guess per account), and reconciling
// the account through a date at or after the checkpoint's retires it — the
// statement has now said what the balance was, and a guess is no longer needed.
package provisional

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Tag is attached to every checkpoint so it is findable from the ledger's own
// filters, without the user having to have thought of it in advance.
const Tag = "reconciliation-checkpoint"

// Of returns the checkpoints on one account, oldest first.
func Of(accountID string, txns []domain.Transaction) []domain.Transaction {
	var out []domain.Transaction
	for _, t := range txns {
		if t.AccountID == accountID && t.IsBalanceCheckpoint() {
			out = append(out, t)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].BalanceCheckpointAt.Equal(out[j].BalanceCheckpointAt) {
			return out[i].BalanceCheckpointAt.Before(out[j].BalanceCheckpointAt)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Open returns the account's current checkpoint — the newest one — and whether
// it has any.
//
// There is only ever meant to be one. If several exist (an older build, an
// import, a sync merge), the newest is the live one and the rest are stale;
// Stale names them so a caller can clear them out.
func Open(accountID string, txns []domain.Transaction) (domain.Transaction, bool) {
	all := Of(accountID, txns)
	if len(all) == 0 {
		return domain.Transaction{}, false
	}
	return all[len(all)-1], true
}

// Stale returns every checkpoint on the account except the newest.
func Stale(accountID string, txns []domain.Transaction) []domain.Transaction {
	all := Of(accountID, txns)
	if len(all) < 2 {
		return nil
	}
	return all[:len(all)-1]
}

// New builds the checkpoint row that moves an account from currentMinor to
// targetMinor as of a date.
//
// It reports ok=false when there is nothing to record: a target equal to the
// current balance needs no adjustment, and writing a zero-amount row would leave
// a permanent artefact saying nothing.
//
// The row is marked excluded from reports here rather than by the caller, because
// "the user has to remember to tick a box" is the failure this replaces.
func New(id, accountID string, currentMinor, targetMinor int64, currency string, asOf, postedAt time.Time) (domain.Transaction, bool) {
	delta := targetMinor - currentMinor
	if delta == 0 {
		return domain.Transaction{}, false
	}
	if asOf.IsZero() {
		asOf = postedAt
	}
	return domain.Transaction{
		ID: id, AccountID: accountID,
		// Posted on the as-of date, not the day it was typed: a balance read on
		// Friday and entered on Monday belongs to Friday, or every figure computed
		// over a date range puts it in the wrong one.
		Date:                asOf,
		Amount:              money.New(delta, currency),
		BalanceCheckpointAt: asOf,
		Tags:                []string{Tag},
		// Real for balances, invisible to reporting — the two halves of what a
		// checkpoint IS, and neither is the user's job to remember.
		ExcludeFromReports: true,
		Cleared:            false,
		Reviewed:           true,
		Source:             domain.TxnSourceManual,
	}, true
}

// SupersededBy returns the ids of checkpoints on an account that a
// reconciliation through the given date has retired.
//
// Reconciling says what the balance actually was, so any guess standing in for it
// up to that date has done its job. A checkpoint dated AFTER the reconciliation
// survives: it is a guess about a period the statement has not covered yet.
func SupersededBy(accountID string, txns []domain.Transaction, through time.Time) []string {
	var out []string
	for _, t := range Of(accountID, txns) {
		if !t.BalanceCheckpointAt.After(dayEnd(through)) {
			out = append(out, t.ID)
		}
	}
	return out
}

// dayEnd pushes a date to the last instant of its calendar day, so a checkpoint
// timestamped mid-afternoon on the statement's closing date counts as covered by
// it. Statement dates are date-only; transaction dates carry a time.
func dayEnd(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 23, 59, 59, int(time.Second-time.Nanosecond), t.Location())
}

// TotalMinor sums the checkpoints in a set — how much of a balance is standing on
// a guess rather than on transactions. Reports and cash-flow figures exclude these
// rows, so this is the number that explains why a balance and a period's activity
// do not add up.
func TotalMinor(txns []domain.Transaction) int64 {
	var sum int64
	for _, t := range txns {
		if t.IsBalanceCheckpoint() {
			sum += t.Amount.Amount
		}
	}
	return sum
}
