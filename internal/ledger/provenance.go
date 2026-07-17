// SPDX-License-Identifier: MIT

package ledger

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// ProvenanceKind classifies how an account's balance most recently came to be
// what it is — the newest transaction on the account decides.
type ProvenanceKind string

const (
	// ProvenanceOpening: no transactions — the balance is the opening balance.
	ProvenanceOpening ProvenanceKind = "opening"
	// ProvenanceAdjusted: the newest movement is an update-balance adjustment
	// (the user forced the balance to a figure).
	ProvenanceAdjusted ProvenanceKind = "adjusted"
	// ProvenanceImported: the newest movement came from a file import or a
	// scanned document.
	ProvenanceImported ProvenanceKind = "imported"
	// ProvenanceDerived: the newest movement is an ordinary ledger row — the
	// balance is transaction-derived.
	ProvenanceDerived ProvenanceKind = "derived"
)

// BalanceProvenance reports how the account's balance most recently moved and
// when. The newest transaction (by date; ties broken by later slice position)
// decides the kind. isAdjustment identifies update-balance adjustment rows —
// a caller-supplied predicate because adjustments are marked at the UI layer
// (description text), not structurally; pass nil to skip that classification.
func BalanceProvenance(accountID string, txns []domain.Transaction, isAdjustment func(domain.Transaction) bool) (ProvenanceKind, time.Time) {
	var newest domain.Transaction
	found := false
	for _, t := range txns {
		if t.AccountID != accountID {
			continue
		}
		if !found || !t.Date.Before(newest.Date) {
			newest, found = t, true
		}
	}
	if !found {
		return ProvenanceOpening, time.Time{}
	}
	switch {
	case isAdjustment != nil && isAdjustment(newest):
		return ProvenanceAdjusted, newest.Date
	case newest.Source == domain.TxnSourceImported || newest.Source == domain.TxnSourceScanned:
		return ProvenanceImported, newest.Date
	default:
		return ProvenanceDerived, newest.Date
	}
}
