// SPDX-License-Identifier: MIT

// Package receiptsplit turns the line items extracted from a receipt into a
// PROPOSED category breakdown for an existing atomic transaction (XC11). It is
// pure — no syscall/js, no rules-engine or store dependency — so it unit-tests on
// native Go: the caller passes a MatchFunc that resolves a line-item name to a
// category id (wired to the rules engine at the edge), and this package handles
// grouping, sign alignment, and the split invariant.
//
// The output is a []domain.CategorySplit that satisfies domain.SplitsReconcile
// against the transaction's amount BY CONSTRUCTION: matched line items are grouped
// onto their category, and the leftover — tax, shipping, rounding, and any
// unmatched lines — is auto-balanced onto the transaction's own category so
// Σ(splits) == txn amount exactly. The proposal is preview-then-approve: it feeds
// the shipped split editor where the user reviews and saves; it is never applied
// directly.
package receiptsplit

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// LineItem is one extracted receipt line: a display name and its printed cost as a
// positive magnitude (the sign is taken from the target transaction, so callers
// pass the amount as it reads on the receipt).
type LineItem struct {
	Name   string
	Amount money.Money
}

// Target is the existing transaction the split is proposed for: its signed amount
// (an expense is negative) and its own category, onto which the remainder lands.
type Target struct {
	Amount     money.Money
	CategoryID string
}

// MatchFunc resolves a line-item name to an existing category id, or "" when no
// category is confidently assigned. The caller wires this to the rules engine
// (rules.FirstMatch / appstate categorization); keeping it a function parameter
// keeps this package pure and independently testable.
type MatchFunc func(name string) string

// Proposal is the result of Propose: the reconciled split lines plus a
// plain-English note describing anything the user should double-check (an unmatched
// remainder, or a receipt-vs-transaction total mismatch). Matched counts how many
// line items were assigned to a category other than the transaction's own, and
// Remainder is the signed minor amount auto-balanced onto the transaction's
// category.
type Proposal struct {
	Splits    []domain.CategorySplit
	Note      string
	Matched   int
	Remainder int64
}

// Propose maps each line item to a category (via match), groups items that share a
// category, and auto-balances the remainder onto target.CategoryID so the returned
// splits sum exactly to target.Amount. The second result is false — and no proposal
// is made — when there is nothing useful to propose: no line items, a currency
// mismatch between any line and the transaction, or when no line matched a category
// (a single all-remainder split is no better than leaving the transaction unsplit).
func Propose(items []LineItem, target Target, match MatchFunc) (Proposal, bool) {
	if len(items) == 0 || match == nil {
		return Proposal{}, false
	}

	cur := target.Amount.Currency

	// Currency guardrail: any line printed in a different currency than the
	// transaction is not something we can safely reconcile — propose nothing.
	for _, it := range items {
		if it.Amount.Currency != "" && it.Amount.Currency != cur {
			return Proposal{}, false
		}
	}

	// Work in the transaction's sign. An expense transaction is negative, while
	// receipt line costs read positive; align every split to the transaction's sign
	// so the magnitudes add up to the transaction amount rather than cancelling it.
	sign := int64(1)
	if target.Amount.Amount < 0 {
		sign = -1
	}

	// Group matched line magnitudes by category, preserving first-seen order for a
	// stable, explainable proposal. A line whose category is empty or equal to the
	// transaction's own category is folded into the remainder (the parent line).
	order := make([]string, 0, len(items))
	byCat := make(map[string]int64)
	var receiptMag int64 // sum of ALL line magnitudes (for the mismatch note)
	matchedLines := 0
	for _, it := range items {
		mag := abs(it.Amount.Amount)
		receiptMag += mag
		cat := match(it.Name)
		if cat == "" || cat == target.CategoryID {
			continue
		}
		if _, seen := byCat[cat]; !seen {
			order = append(order, cat)
		}
		byCat[cat] += mag
		matchedLines++
	}

	// Nothing matched a distinct category: a lone remainder line on the parent
	// category is no different from an unsplit transaction — don't propose.
	if len(order) == 0 {
		return Proposal{}, false
	}

	txnMag := abs(target.Amount.Amount)
	var matchedMag int64
	for _, c := range order {
		matchedMag += byCat[c]
	}
	// The remainder auto-balances the split to the transaction total: tax, shipping,
	// rounding, and any unmatched lines all land on the transaction's own category.
	remainderMag := txnMag - matchedMag

	splits := make([]domain.CategorySplit, 0, len(order)+1)
	for _, c := range order {
		splits = append(splits, domain.CategorySplit{
			CategoryID: c,
			Amount:     money.New(sign*byCat[c], cur),
		})
	}
	// Emit the remainder line whenever it is non-zero, so Σ(splits) == txn amount.
	if remainderMag != 0 {
		splits = append(splits, domain.CategorySplit{
			CategoryID: target.CategoryID,
			Amount:     money.New(sign*remainderMag, cur),
		})
	}

	p := Proposal{
		Splits:    splits,
		Matched:   matchedLines,
		Remainder: sign * remainderMag,
	}
	p.Note = buildNote(receiptMag, txnMag, matchedMag, remainderMag)
	return p, true
}

// buildNote returns a plain-English caution when the user should double-check the
// proposal: a receipt total that differs from the transaction amount, or a
// remainder that isn't a small tax/shipping-shaped leftover. Empty when the
// breakdown is clean (lines reconcile with only a minor remainder).
func buildNote(receiptMag, txnMag, matchedMag, remainderMag int64) string {
	diff := receiptMag - txnMag
	switch {
	case diff > 0:
		return "The receipt's line items add up to more than this transaction. " +
			"The extra was placed on the transaction's own category — review before saving."
	case diff < 0:
		return "The receipt's line items add up to less than this transaction. " +
			"The difference (tax, shipping, or an unread line) is on the transaction's own category."
	case remainderMag != 0:
		// receipt total matches the txn but matched lines don't cover it — unmatched
		// lines were folded into the remainder.
		return "Some lines weren't matched to a category and were placed on the " +
			"transaction's own category — review before saving."
	default:
		return ""
	}
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
