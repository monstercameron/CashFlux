// SPDX-License-Identifier: MIT

package reports

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/txnlinks"
)

// refundLinks holds the refund-pair links (XC2) whose netting the reporting core
// folds into period totals and category totals. Read-model overlay only — the
// ledger atoms are never rewritten. Order-group links change no totals and are
// ignored. Installed by SetRefundLinks; nil disables netting.
var refundLinks []domain.TxnLink

// SetRefundLinks installs the refund-pair links whose per-period netting the
// reports apply. Pass the app's full link set (order groups are filtered out);
// call again when links change, or with nil to disable.
//
// With these installed, a March purchase returned in April reads as net spend in
// March's income-vs-expense and category totals, and April shows neither
// inflated income nor a phantom negative.
func SetRefundLinks(links []domain.TxnLink) {
	filtered := links[:0:0]
	for _, l := range links {
		if l.Kind == domain.TxnLinkRefundPair {
			filtered = append(filtered, l)
		}
	}
	refundLinks = filtered
}

// netted folds installed refund-pair netting into txns for report aggregation,
// returning the input unchanged when no refund links are set. This is the single
// hook the reporting core (IncomeVsExpense, categoryTotals) calls.
func netted(txns []domain.Transaction) []domain.Transaction {
	if len(refundLinks) == 0 {
		return txns
	}
	return txnlinks.NetTransactions(txns, refundLinks)
}
