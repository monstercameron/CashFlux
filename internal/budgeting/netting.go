// SPDX-License-Identifier: MIT

package budgeting

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/txnlinks"
)

// refundLinks holds the refund-pair links (XC2) whose netting budget spend
// evaluation folds in. It is a read-model overlay only: the ledger atoms are
// never rewritten. Order-group links are ignored here — grouping is presentation
// and changes no totals. Installed by SetRefundLinks; nil disables netting.
var refundLinks []domain.TxnLink

// Deprecated: pass NewNettedLedgerWithLinks in ViewInput.Ledger instead.
// SetRefundLinks installs the refund-pair links whose per-period netting budget
// spend evaluation applies. Pass the app's full link set (order groups are
// filtered out internally); call again when links change, or with nil to disable.
//
// With these installed, a return posted in April against a purchase in March is
// netted into March's budget (the purchase reads as net spend) and contributes
// nothing to April — no phantom negative. The ledger keeps both transactions.
func SetRefundLinks(links []domain.TxnLink) {
	filtered := links[:0:0]
	for _, l := range links {
		if l.Kind == domain.TxnLinkRefundPair {
			filtered = append(filtered, l)
		}
	}
	refundLinks = filtered
}

// nettedForSpending folds installed refund-pair netting into txns for budget
// spend evaluation, returning the input unchanged when no refund links are set.
// This is the single hook spentCovered calls.
func nettedForSpending(txns []domain.Transaction) []domain.Transaction {
	if len(refundLinks) == 0 {
		return txns
	}
	return txnlinks.NetTransactions(txns, refundLinks)
}

// NewNettedLedgerWithLinks creates a scope-local refund read model. An explicit
// nil links slice means no refunds, independent of any legacy SetRefundLinks
// caller. Pass this ledger in ViewInput.Ledger for deterministic evaluation.
func NewNettedLedgerWithLinks(all []domain.Transaction, links []domain.TxnLink) NettedLedger {
	return NettedLedger{transactions: txnlinks.NetTransactions(all, links)}
}
