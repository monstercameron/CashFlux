// SPDX-License-Identifier: MIT

// Package ordermatch prepares order-import matches for a trustworthy review.
package ordermatch

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/orderimport"
)

// ReviewMatch pairs a matcher's verdict with the ledger transactions it names.
// MissingTxnIDs is retained so a stale match cannot look complete when a
// transaction disappears between matching and rendering.
type ReviewMatch struct {
	Match         orderimport.Match
	Charges       []domain.Transaction
	MissingTxnIDs []string
}

// ForReview resolves every matched transaction in matcher order while
// preserving the input order of orders. It does not infer or alter a match.
func ForReview(matches []orderimport.Match, transactions []domain.Transaction) []ReviewMatch {
	byID := make(map[string]domain.Transaction, len(transactions))
	for _, txn := range transactions {
		byID[txn.ID] = txn
	}

	rows := make([]ReviewMatch, len(matches))
	for i, match := range matches {
		row := ReviewMatch{Match: match}
		for _, id := range match.TxnIDs {
			txn, ok := byID[id]
			if !ok {
				row.MissingTxnIDs = append(row.MissingTxnIDs, id)
				continue
			}
			row.Charges = append(row.Charges, txn)
		}
		rows[i] = row
	}
	return rows
}
