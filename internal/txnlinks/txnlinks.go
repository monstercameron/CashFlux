// SPDX-License-Identifier: MIT

// Package txnlinks is the pure logic for transaction-to-transaction links
// (domain.TxnLink): order grouping (XC1) and refund pairing (XC2).
//
// It answers three questions, each a deterministic function over transactions
// and links with no platform dependencies (unit-tested on native Go):
//
//   - Grouping: which order-group does a transaction belong to, and how does the
//     group's member sum reconcile against an entered order total.
//   - Refund netting: how much does a refund pair net, and per transaction, what
//     read-model adjustment folds the refund into the ORIGINAL purchase's period
//     without touching the ledger atoms (the single-source rule).
//   - Candidate detection: which prior purchases could a positive transaction be
//     a refund of (same payee, amount within the original, within a window).
package txnlinks

import (
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// RefundWindowDays is how far back (in days) a refund may pair to its original
// purchase for candidate detection (XC2: within 90 days).
const RefundWindowDays = 90

// GroupOf returns the order-group link a transaction belongs to, or ok=false.
// A transaction belongs to at most one order-group (enforced on write), so the
// first match wins.
func GroupOf(txnID string, links []domain.TxnLink) (domain.TxnLink, bool) {
	for _, l := range links {
		if l.Kind == domain.TxnLinkOrderGroup && l.HasTxn(txnID) {
			return l, true
		}
	}
	return domain.TxnLink{}, false
}

// GroupsByTxn indexes every grouped transaction id to its order-group link, for
// a single-pass render of a transaction list.
func GroupsByTxn(links []domain.TxnLink) map[string]domain.TxnLink {
	out := map[string]domain.TxnLink{}
	for _, l := range links {
		if l.Kind != domain.TxnLinkOrderGroup {
			continue
		}
		for _, id := range l.TxnIDs {
			out[id] = l
		}
	}
	return out
}

// GroupMembers returns the group's member transactions in link order (skipping
// ids not present in txns). byID should map transaction id to transaction.
func GroupMembers(link domain.TxnLink, byID map[string]domain.Transaction) []domain.Transaction {
	out := make([]domain.Transaction, 0, len(link.TxnIDs))
	for _, id := range link.TxnIDs {
		if t, ok := byID[id]; ok {
			out = append(out, t)
		}
	}
	return out
}

// GroupSum totals the group's member amounts (by minor unit, in the first
// member's currency). Amounts keep their sign, so three card charges sum to the
// order's negative total.
func GroupSum(members []domain.Transaction) money.Money {
	if len(members) == 0 {
		return money.Money{}
	}
	sum := money.Zero(members[0].Amount.Currency)
	for _, t := range members {
		sum.Amount += t.Amount.Amount
	}
	return sum
}

// Reconcile compares a group's member sum against an entered order total (both
// as magnitudes) and reports the remainder left to assign, mirroring the split
// editor's Balanced / left-to-assign line. A zero enteredTotal means none was
// entered, so remainder is zero and Balanced is true (nothing to reconcile).
//
// remainder = |enteredTotal| - |sum|: positive means the members don't yet cover
// the order (money still unaccounted), negative means they overshoot it.
func Reconcile(sum, enteredTotal money.Money) (remainder money.Money, balanced bool) {
	cur := sum.Currency
	if cur == "" {
		cur = enteredTotal.Currency
	}
	if enteredTotal.Amount == 0 {
		return money.Zero(cur), true
	}
	rem := abs(enteredTotal.Amount) - abs(sum.Amount)
	return money.New(rem, cur), rem == 0
}

// RefundNet returns the amount a refund pair nets in the ORIGINAL purchase's
// period, as a positive magnitude in the original's currency. A non-zero
// link.Amount is the explicit (partial) netted amount; zero means "full" — net
// the refund transaction's own amount. The result is capped at the original's
// magnitude so a refund can never flip the purchase into income.
//
// original and refund are the pair's two transactions (link.TxnIDs[0] and [1]).
func RefundNet(link domain.TxnLink, original, refund domain.Transaction) money.Money {
	cur := original.Amount.Currency
	n := abs(link.Amount.Amount)
	if n == 0 {
		n = abs(refund.Amount.Amount)
	}
	if cap := abs(original.Amount.Amount); n > cap {
		n = cap
	}
	return money.New(n, cur)
}

// NetAdjustments returns the per-transaction read-model adjustment for refund
// pairs: a signed amount to ADD to a transaction's own amount so budgets and
// reports see the refund netted in the ORIGINAL purchase's month, while the
// ledger atoms stay untouched (single-source rule).
//
// For each refund pair the original purchase (negative expense) gains +net
// (its magnitude shrinks toward zero — net spend in its own month) and the
// refund transaction (positive) gains -net (it zeroes out in its month, so no
// phantom income and no phantom negative). Order-group links contribute nothing:
// grouping is presentation only and changes no totals.
//
// The map only contains transactions that are actually adjusted. Callers apply
// it via NetTransactions (or read it directly for explainability).
func NetAdjustments(txns []domain.Transaction, links []domain.TxnLink) map[string]money.Money {
	byID := make(map[string]domain.Transaction, len(txns))
	for _, t := range txns {
		byID[t.ID] = t
	}
	out := map[string]money.Money{}
	for _, l := range links {
		if l.Kind != domain.TxnLinkRefundPair || len(l.TxnIDs) != 2 {
			continue
		}
		original, ok := byID[l.TxnIDs[0]]
		if !ok {
			continue
		}
		refund, ok := byID[l.TxnIDs[1]]
		if !ok {
			continue
		}
		net := RefundNet(l, original, refund)
		if net.Amount == 0 {
			continue
		}
		out[original.ID] = addMinor(out[original.ID], net.Amount, net.Currency)
		out[refund.ID] = addMinor(out[refund.ID], -net.Amount, refund.Amount.Currency)
	}
	return out
}

// NetTransactions returns a copy of txns with refund-pair netting folded into
// each affected transaction's amount, for read-model consumers (budgets and
// reports). The input slice and its transactions are not mutated. This is the
// single adapter both budgeting/netting.go and reports/netting.go call.
func NetTransactions(txns []domain.Transaction, links []domain.TxnLink) []domain.Transaction {
	adj := NetAdjustments(txns, links)
	if len(adj) == 0 {
		return txns
	}
	out := make([]domain.Transaction, len(txns))
	copy(out, txns)
	for i := range out {
		a, ok := adj[out[i].ID]
		if !ok {
			continue
		}
		out[i].Amount = addMinor(out[i].Amount, a.Amount, out[i].Amount.Currency)
	}
	return out
}

// RefundCandidates returns prior purchases a positive refund transaction could
// pair to, best match first: same payee (or description when payee is blank),
// an expense whose magnitude is at least the refund amount (amount ≤ original),
// dated on or before the refund and within RefundWindowDays. Already-linked
// originals are excluded when their link is passed in linked.
//
// The result is ordered by closest amount first, then most-recent purchase, so
// the obvious match surfaces at the top of the picker.
func RefundCandidates(refund domain.Transaction, txns []domain.Transaction, links []domain.TxnLink) []domain.Transaction {
	if !refund.IsIncome() {
		return nil
	}
	linkedOriginals := map[string]bool{}
	for _, l := range links {
		if l.Kind == domain.TxnLinkRefundPair && len(l.TxnIDs) > 0 {
			linkedOriginals[l.TxnIDs[0]] = true
		}
	}
	key := matchKey(refund)
	window := refund.Date.AddDate(0, 0, -RefundWindowDays)
	refAmt := abs(refund.Amount.Amount)
	var out []domain.Transaction
	for _, t := range txns {
		if t.ID == refund.ID || !t.IsExpense() {
			continue
		}
		if linkedOriginals[t.ID] {
			continue
		}
		if matchKey(t) != key {
			continue
		}
		if abs(t.Amount.Amount) < refAmt {
			continue // refund exceeds the original — not a match
		}
		if t.Date.After(refund.Date) || t.Date.Before(window) {
			continue
		}
		out = append(out, t)
	}
	sort.SliceStable(out, func(i, j int) bool {
		di := abs(abs(out[i].Amount.Amount) - refAmt)
		dj := abs(abs(out[j].Amount.Amount) - refAmt)
		if di != dj {
			return di < dj
		}
		return out[i].Date.After(out[j].Date)
	})
	return out
}

// PairOf returns the refund-pair link a transaction is part of, or ok=false.
func PairOf(txnID string, links []domain.TxnLink) (domain.TxnLink, bool) {
	for _, l := range links {
		if l.Kind == domain.TxnLinkRefundPair && l.HasTxn(txnID) {
			return l, true
		}
	}
	return domain.TxnLink{}, false
}

// matchKey is the payee-or-description signature two transactions must share to
// be a refund candidate, normalized (trimmed, lower-cased) so casing/whitespace
// don't split an obvious match.
func matchKey(t domain.Transaction) string {
	k := t.Payee
	if strings.TrimSpace(k) == "" {
		k = t.Desc
	}
	return strings.ToLower(strings.TrimSpace(k))
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// addMinor adds delta minor units to m, defaulting m's currency to cur when m
// is the zero value (so an accumulator starts in the right currency).
func addMinor(m money.Money, delta int64, cur string) money.Money {
	if m.Currency == "" {
		m.Currency = cur
	}
	m.Amount += delta
	return m
}
