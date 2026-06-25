// SPDX-License-Identifier: MIT

// Package quickpayee provides a pure-Go helper for deriving a recent-payee
// suggestion list for the Quick-Add transaction form. It contains no syscall/js
// and imports only internal/domain and stdlib, so it compiles and unit-tests on
// native Go.
package quickpayee

import (
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// maxResults is the hard cap on the number of payee suggestions returned.
const maxResults = 20

// RecentPayees returns up to 20 distinct payee strings derived from the most
// recent transactions.
//
// Algorithm:
//  1. A copy of txns is sorted by Date descending so that the most recent
//     transactions are visited first. The original slice is never mutated.
//  2. For each transaction, the effective label is Payee when non-empty; otherwise
//     Desc is used as a fallback (so transactions without an explicit payee still
//     contribute a recognisable name).
//  3. Deduplication is case-insensitive: the first occurrence of a label (in
//     most-recent-first order) wins and its original casing is preserved. Later
//     entries with the same label under a different casing are skipped.
//  4. The result is capped at 20 entries. Passing n ≤ 0 scans all transactions
//     before the cap; a positive n limits the input scan to the first n
//     (most-recent) transactions — useful when callers want to restrict recency
//     further.
//
// The returned slice is ordered most-recent-first (the order in which distinct
// labels were first encountered during the date-descending scan).
func RecentPayees(txns []domain.Transaction, n int) []string {
	if len(txns) == 0 {
		return nil
	}

	// Sort a copy by Date descending — most recent first.
	sorted := make([]domain.Transaction, len(txns))
	copy(sorted, txns)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Date.After(sorted[j].Date)
	})

	// Limit the scan window when n is a positive bound.
	if n > 0 && n < len(sorted) {
		sorted = sorted[:n]
	}

	seen := make(map[string]struct{})
	result := make([]string, 0, maxResults)

	for _, t := range sorted {
		label := t.Payee
		if label == "" {
			label = t.Desc
		}
		if label == "" {
			continue
		}
		key := strings.ToLower(label)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, label)
		if len(result) == maxResults {
			break
		}
	}

	return result
}
