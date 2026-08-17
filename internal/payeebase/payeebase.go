// SPDX-License-Identifier: MIT

// Package payeebase computes what a merchant normally charges, so a single
// transaction can be judged against that merchant's own history rather than
// against a household-wide threshold.
//
// The notification feed already had this idea buried inside its unusual-charge
// generator. It is pulled out here because a second caller needs it: a workflow
// condition cannot say "this subscription went up" without a baseline to compare
// against, and there is no honest way to express "my streaming bill changed"
// with a fixed dollar threshold — the whole point is that the number that
// matters is different for every merchant (C405).
//
// The baseline is a MEDIAN, not a mean, because a merchant with one annual
// renewal among eleven monthly charges has a mean nobody would recognize. And it
// always excludes the transaction being judged: a charge that is its own
// baseline can never look unusual.
package payeebase

import (
	"slices"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// MinHistory is how many OTHER expenses at the same merchant are needed before
// its baseline means anything. Below this there is no established normal to
// deviate from, so callers should decline to judge rather than guess.
const MinHistory = 3

// Key normalizes a transaction to the merchant it belongs to: its Payee if set,
// otherwise its Desc, lower-cased and trimmed. Empty when neither is set —
// such a transaction cannot be grouped and should be skipped, not lumped into a
// shared empty-string bucket.
func Key(t domain.Transaction) string {
	s := strings.TrimSpace(t.Payee)
	if s == "" {
		s = strings.TrimSpace(t.Desc)
	}
	return strings.ToLower(s)
}

// Median returns the median of a slice of minor-unit magnitudes, or 0 for an
// empty slice. The input is not modified.
func Median(vals []int64) int64 {
	if len(vals) == 0 {
		return 0
	}
	s := make([]int64, len(vals))
	copy(s, vals)
	slices.Sort(s)
	mid := len(s) / 2
	if len(s)%2 == 1 {
		return s[mid]
	}
	return (s[mid-1] + s[mid]) / 2
}

// Typical reports the median magnitude, in minor units, of the OTHER expenses
// sharing target's merchant key — the amount this merchant normally charges —
// and whether there was enough history (at least MinHistory others) to say.
//
// Comparison is on raw minor units of each transaction's own currency. Callers
// mixing currencies must convert before calling; a multi-currency merchant is
// rare enough that quietly averaging across currencies would be a worse error
// than requiring the caller to be explicit.
//
// Matching excludes target by ID, so passing a transaction that is itself in
// txns is the normal case and does not skew its own baseline.
func Typical(txns []domain.Transaction, target domain.Transaction) (int64, bool) {
	key := Key(target)
	if key == "" {
		return 0, false
	}
	var others []int64
	for _, t := range txns {
		if t.ID == target.ID || !t.IsExpense() || Key(t) != key {
			continue
		}
		others = append(others, t.Amount.Abs().Amount)
	}
	if len(others) < MinHistory {
		return 0, false
	}
	m := Median(others)
	return m, m > 0
}

// Ratio reports this transaction's magnitude as a multiple of its merchant's
// typical charge — 1.0 is business as usual, 1.5 is half again as much, 0.5 is
// half. The second result is false when there is not enough history, which
// callers must treat as "no opinion" rather than as a ratio of zero: an
// unknown baseline is not evidence of a change.
func Ratio(txns []domain.Transaction, target domain.Transaction) (float64, bool) {
	typical, ok := Typical(txns, target)
	if !ok {
		return 0, false
	}
	return float64(target.Amount.Abs().Amount) / float64(typical), true
}
