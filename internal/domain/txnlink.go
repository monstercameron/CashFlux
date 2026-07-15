// SPDX-License-Identifier: MIT

package domain

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/money"
)

// TxnLinkKind names the relationship a TxnLink records between transactions.
// It is the generalized transaction-link primitive (XC0b): one persisted
// relation with a kind enum, so order grouping, refund pairing, and later
// bill-matching all reuse a single data model instead of three bespoke ones.
type TxnLinkKind string

const (
	// TxnLinkOrderGroup ties N transactions into one logical purchase (XC1) —
	// an online order that shipped in several boxes and posted as several card
	// charges, none matching the order total. Members are ordered; the first is
	// the primary/original. A group needs at least two members.
	TxnLinkOrderGroup TxnLinkKind = "order-group"

	// TxnLinkRefundPair ties a refund (or reimbursement) transaction to the
	// original purchase it offsets (XC2). Exactly two members: the first is the
	// original purchase, the second is the refund. Read models net the pair in
	// the ORIGINAL transaction's period. Amount may carry a partial-refund value.
	TxnLinkRefundPair TxnLinkKind = "refund-pair"

	// TxnLinkBillMatch is reserved for a future bill-to-payment match (the
	// recurring/bills work). Declared here so the enum is designed to grow
	// without a schema change; no logic consumes it yet.
	TxnLinkBillMatch TxnLinkKind = "bill-match"
)

// KnownTxnLinkKind reports whether k is a kind this build understands.
func KnownTxnLinkKind(k TxnLinkKind) bool {
	switch k {
	case TxnLinkOrderGroup, TxnLinkRefundPair, TxnLinkBillMatch:
		return true
	default:
		return false
	}
}

// TxnLink is a persisted relation among transactions. It never mutates or owns
// the transactions it references — grouping and pairing are read-model overlays,
// so deleting a link releases its members without deleting any transaction.
//
// Invariants (enforced at the appstate write seam, see appstate.PutTxnLink):
//   - Kind is a known TxnLinkKind.
//   - An order-group has >= 2 TxnIDs; a refund-pair has exactly 2.
//   - A transaction belongs to at most one order-group.
//   - TxnIDs is ordered; TxnIDs[0] is the primary/original transaction.
type TxnLink struct {
	// ID is the stable identifier for the link row.
	ID string `json:"id"`
	// Kind is the relationship this link records.
	Kind TxnLinkKind `json:"kind"`
	// TxnIDs are the linked transactions, in order. The first is the primary
	// (an order group's anchor, or a refund pair's original purchase).
	TxnIDs []string `json:"txnIds"`
	// Amount is the netted amount for a partial refund pair (the money the
	// refund actually returns). Zero means "full" — net the refund's own amount.
	// Unused by order groups.
	Amount money.Money `json:"amount,omitempty"`
	// Note is an optional user label (e.g. an order number, or why a pair was made).
	Note string `json:"note,omitempty"`
	// EnteredTotal is the order total a user typed for an order group, so the
	// band can reconcile the member sum against it (remainder line). Zero means
	// none entered. Unused by refund pairs.
	EnteredTotal money.Money `json:"enteredTotal,omitempty"`
	// CreatedAt is when the link was made.
	CreatedAt time.Time `json:"createdAt"`
}

// Primary returns the primary/original transaction id (TxnIDs[0]) or "" if the
// link has no members.
func (l TxnLink) Primary() string {
	if len(l.TxnIDs) == 0 {
		return ""
	}
	return l.TxnIDs[0]
}

// HasTxn reports whether the given transaction id is a member of the link.
func (l TxnLink) HasTxn(txnID string) bool {
	for _, id := range l.TxnIDs {
		if id == txnID {
			return true
		}
	}
	return false
}
