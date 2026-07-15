// SPDX-License-Identifier: MIT

package domain

import "github.com/monstercameron/CashFlux/internal/money"

// CategorySplit assigns part of a transaction's amount to a category. A
// transaction with one bank charge but several categorized lines (a grocery
// receipt: produce, dairy, household) carries its breakdown as Splits, so it
// counts once against the account yet reports per-category spend.
//
// # The split contract — who attributes per line, and who must not
//
// A split transaction is ONE cash movement with MANY category meanings. Every
// consumer of transactions falls on one side of that line, and getting a
// consumer on the wrong side either double-counts money or hides it:
//
// Cash-side consumers read the WHOLE transaction and must IGNORE Splits —
// the charge hit the account exactly once:
//   - account/cleared/running balances + net worth (ledger.Balance and friends)
//   - duplicate detection (dedupe.Signature keys on the parent's date+amount+desc)
//   - transfers, forecasting, reconcile — anything answering "where is the money"
//
// Category-side consumers answer "what was the money FOR" and MUST attribute
// each split line to its own category (falling back to the whole-transaction
// CategoryID only when there are no splits — the pattern in each site below):
//   - budget consumption: budgeting.spentCovered (feeds Spent / Evaluate /
//     EvaluateRollup — the budget bars and over-limit checks)
//   - report aggregation: reports.categoryTotals (feeds SpendingByCategory,
//     Trends, Rollup, TopMovers)
//   - the UI surfaces that display the breakdown (row badge, category-cell CSV
//     list, the split editors)
//
// Writers must keep Σ(Splits) == Amount (SplitsReconcile): the split editor
// gates Save on it, the receipt importer validates it, and the transaction
// edit form blocks an amount change that would desync an existing breakdown.
// Merge (fingerprint) carries a breakdown onto the surviving duplicate.
//
// # Known gaps (category-side consumers still reading only CategoryID)
//
// Audited 2026-07-14. Each of these misattributes split lines today; fix by
// mirroring the HasSplits branch used in budgeting.spentCovered /
// reports.categoryTotals:
//   - txnfilter.ApplyWithLabels: the category filter matches only the parent's
//     CategoryID, so a split line's category won't surface its transaction in
//     the list — and a budget's drill-through can disagree with its own bar.
//   - smartengine (budgets.go and siblings): re-derives per-category spend
//     directly from CategoryID, so Smart insights can contradict the budget page
//     for split transactions.
//   - reports/deductible.go, income.go, yeartax.go: a deductible/income split
//     line inside a mixed receipt is not counted.
//   - widgetsource: widget-builder category aggregations read CategoryID only.
//   - store/csv.go: CSV export writes one category column — a breakdown is
//     silently dropped on a CSV round-trip (the JSON dataset export keeps it).
type CategorySplit struct {
	CategoryID string      `json:"categoryId"`
	Amount     money.Money `json:"amount"`
}

// SplitsTotal returns the sum of the splits, in the currency of the first split
// (an empty list totals zero with an empty currency). Splits are assumed to share
// a currency; mixed currencies are summed by minor amount only.
func SplitsTotal(splits []CategorySplit) money.Money {
	if len(splits) == 0 {
		return money.Money{}
	}
	total := money.Zero(splits[0].Amount.Currency)
	for _, s := range splits {
		total.Amount += s.Amount.Amount
	}
	return total
}

// SplitsReconcile reports whether the splits sum exactly to amount (to the minor
// unit). A transaction with no splits reconciles trivially — it simply isn't
// split. The comparison is on the integer minor amount, so an unset split
// currency still reconciles against amount when the totals match.
func SplitsReconcile(amount money.Money, splits []CategorySplit) bool {
	if len(splits) == 0 {
		return true
	}
	return SplitsTotal(splits).Amount == amount.Amount
}

// HasSplits reports whether the transaction carries a category breakdown.
func (t Transaction) HasSplits() bool { return len(t.Splits) > 0 }

// SplitsReconcile reports whether this transaction's splits sum to its amount.
func (t Transaction) SplitsReconcile() bool { return SplitsReconcile(t.Amount, t.Splits) }
