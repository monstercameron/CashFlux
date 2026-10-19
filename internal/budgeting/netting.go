// SPDX-License-Identifier: MIT

package budgeting

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/txnlinks"
)

// NettedLedger is a read-only refund-netted transaction view. Build it once for
// a data revision and share it across evaluators. Its slice and nested values
// must not be mutated; evaluators select contributions without copying rows.
// An ordinary transaction slice is assignable when no refund links apply.
type NettedLedger []domain.Transaction

// NewNettedLedger folds explicit refund links into a scope-local read model.
// Passing an existing NettedLedger is idempotent, even with the same links:
// refunds are never applied a second time. Rebuild from raw transactions when
// the data revision or links change. Omitting links means no refund netting.
func NewNettedLedger[T ~[]domain.Transaction](all T, links ...[]domain.TxnLink) NettedLedger {
	if ledger, ok := any(all).(NettedLedger); ok {
		return ledger
	}
	if len(links) == 0 || len(links[0]) == 0 {
		return NettedLedger(all)
	}
	// Only linked rows need the refund resolver's ID index. Indexing the entire
	// ledger as Transaction values would copy hundreds of bytes per row again.
	wanted := make(map[string]bool, 2*len(links[0]))
	for _, link := range links[0] {
		if link.Kind == domain.TxnLinkRefundPair && len(link.TxnIDs) == 2 {
			wanted[link.TxnIDs[0]], wanted[link.TxnIDs[1]] = true, true
		}
	}
	if len(wanted) == 0 {
		return NettedLedger(all)
	}
	linked := make([]domain.Transaction, 0, len(wanted))
	for i := range all {
		if wanted[all[i].ID] {
			linked = append(linked, all[i])
		}
	}
	adjustments := txnlinks.NetAdjustments(linked, links[0])
	if len(adjustments) == 0 {
		return NettedLedger(all)
	}
	out := make(NettedLedger, len(all))
	copy(out, all)
	for i := range out {
		if adjustment, ok := adjustments[out[i].ID]; ok {
			out[i].Amount.Amount += adjustment.Amount
		}
	}
	return out
}

// Transactions returns the read-only ledger view without allocating or copying.
// Keep its NettedLedger type when passing it to another budgeting evaluator so
// reconstructing the read model remains idempotent. Never mutate this view.
func (l NettedLedger) Transactions() NettedLedger { return l }
