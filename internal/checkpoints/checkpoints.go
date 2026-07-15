// SPDX-License-Identifier: MIT

// Package checkpoints builds a per-account balance SERIES anchored on
// user-confirmed balance checkpoints (AC6). It is the local-first fix for sparse
// ledgers: a user who does not enter every transaction accumulates historical
// drift, because a raw transaction fold (opening balance + every txn) diverges
// from what the account actually held.
//
// The anchor is the already-shipped domain.BalanceSnapshot: appstate records one
// automatically whenever an account's balance changes through the update-balance /
// reconcile flow, so every confirmed balance is a trusted "on this date it was
// really this much" point with no new persistence. This package prefers the
// nearest snapshot at or before a date and then applies only the transactions
// since that snapshot — so history is correct back to the last thing the user
// confirmed, and only the pre-first-anchor tail falls back to the plain ledger
// fold.
//
// All math is per-account, in the account's own currency, int64 minor units — no
// floats, no cross-currency mixing. Pure Go, unit-tested on native Go.
package checkpoints

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// dayStart truncates a time to the start of its calendar day in its own location,
// so anchor/transaction/date comparisons are day-granular and deterministic.
func dayStart(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// ForAccount filters balance snapshots to one account and returns them sorted
// ascending by date (stable on equal dates by input order). It is the entry point
// the series functions expect — callers pass the account's snapshot history
// (appstate.BalanceHistory) once.
func ForAccount(all []domain.BalanceSnapshot, accountID string) []domain.BalanceSnapshot {
	var out []domain.BalanceSnapshot
	for _, c := range all {
		if c.AccountID == accountID {
			out = append(out, c)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].AsOf.Before(out[j].AsOf)
	})
	return out
}

// nearestAnchor returns the latest snapshot whose date is on or before day, and
// whether one was found. anchors must be sorted ascending (see ForAccount).
func nearestAnchor(anchors []domain.BalanceSnapshot, day time.Time) (domain.BalanceSnapshot, bool) {
	day = dayStart(day)
	var found domain.BalanceSnapshot
	ok := false
	for _, c := range anchors {
		if !dayStart(c.AsOf).After(day) {
			found, ok = c, true
			continue
		}
		break
	}
	return found, ok
}

// txnDeltaMinor sums the signed amounts of an account's transactions in the
// half-open day window (after, upto] — strictly after the anchor day and on or
// before the target day. Transfers are included: a transfer leg genuinely moves
// this account's balance even though it is excluded from income/expense.
func txnDeltaMinor(txns []domain.Transaction, accountID string, after, upto time.Time) int64 {
	after, upto = dayStart(after), dayStart(upto)
	var sum int64
	for _, t := range txns {
		if t.AccountID != accountID {
			continue
		}
		d := dayStart(t.Date)
		if d.After(after) && !d.After(upto) {
			sum += t.Amount.Amount
		}
	}
	return sum
}

// ledgerFoldMinor returns the plain opening-balance + txns-through-day fold for an
// account (the pre-anchor fallback). Amounts are assumed to be in the account's
// currency; a mismatched opening-balance currency is treated as zero opening.
func ledgerFoldMinor(acc domain.Account, txns []domain.Transaction, day time.Time) int64 {
	day = dayStart(day)
	var sum int64
	if acc.OpeningBalance.Currency == "" || acc.OpeningBalance.Currency == acc.Currency {
		sum = acc.OpeningBalance.Amount
	}
	for _, t := range txns {
		if t.AccountID != acc.ID {
			continue
		}
		if !dayStart(t.Date).After(day) {
			sum += t.Amount.Amount
		}
	}
	return sum
}

// BalanceMinorAt returns the account's balance (minor units) at day, preferring
// the nearest checkpoint at or before day and applying transactions since it. When
// no checkpoint precedes day, it falls back to the plain ledger fold. anchors must
// be this account's snapshots (use ForAccount); txns may be the whole set.
func BalanceMinorAt(acc domain.Account, txns []domain.Transaction, anchors []domain.BalanceSnapshot, day time.Time) int64 {
	if anchor, ok := nearestAnchor(anchors, day); ok {
		return anchor.BalanceMinor + txnDeltaMinor(txns, acc.ID, anchor.AsOf, day)
	}
	return ledgerFoldMinor(acc, txns, day)
}

// BalanceAt returns BalanceMinorAt as a money.Money in the account's currency.
func BalanceAt(acc domain.Account, txns []domain.Transaction, anchors []domain.BalanceSnapshot, day time.Time) money.Money {
	return money.Money{Amount: BalanceMinorAt(acc, txns, anchors, day), Currency: acc.Currency}
}

// Point is one dated sample of an account's anchored balance series.
type Point struct {
	Date         time.Time
	BalanceMinor int64
}

// Series returns a daily anchored balance series for an account across the
// inclusive day range [start, end], one Point per calendar day. It is the input to
// AC2's per-account sparkline: because it is anchor-aware, a flat stretch since the
// last confirmed checkpoint is honest rather than an artifact of missing txns. The
// range is clamped so start is not after end; an empty range yields nil.
func Series(acc domain.Account, txns []domain.Transaction, anchors []domain.BalanceSnapshot, start, end time.Time) []Point {
	start, end = dayStart(start), dayStart(end)
	if start.After(end) {
		return nil
	}
	var out []Point
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		out = append(out, Point{Date: d, BalanceMinor: BalanceMinorAt(acc, txns, anchors, d)})
	}
	return out
}

// NetWorthMinorAt sums each account's anchored balance at day, treating liability
// balances as negative. history is the whole snapshot set (filtered per account
// internally). All accounts are assumed to share one base currency — callers doing
// multi-currency net worth should convert per account before summing; this helper
// is the single-currency net-worth series companion to the per-account series and
// keeps AC6's anchoring for the household total where currencies already match.
func NetWorthMinorAt(accounts []domain.Account, txns []domain.Transaction, history []domain.BalanceSnapshot, day time.Time) int64 {
	var sum int64
	for _, a := range accounts {
		if a.Archived {
			continue
		}
		bal := BalanceMinorAt(a, txns, ForAccount(history, a.ID), day)
		if a.IsLiability() {
			sum -= bal
		} else {
			sum += bal
		}
	}
	return sum
}
