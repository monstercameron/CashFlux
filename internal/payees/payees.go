// SPDX-License-Identifier: MIT

// Package payees provides helpers for extracting and ranking payee names from
// a transaction history. It has no syscall/js dependency and is fully unit-testable
// on native Go.
package payees

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// RecentPayees returns up to limit distinct payee strings drawn from txns, ordered
// by the date of their most-recent occurrence (newest first). The Payee field is
// preferred; when it is blank the Desc field is used as a fallback, mirroring the
// way rules match transactions. Blank strings and pure-whitespace values are
// skipped. Comparison is case-insensitive for deduplication, but the first-seen
// casing (most recent) is preserved in the output.
//
// If limit ≤ 0 the function returns all distinct payees.
func RecentPayees(txns []domain.Transaction, limit int) []string {
	// Sort-free approach: iterate newest-first (callers typically get transactions
	// in insertion order; we do a single O(n) pass and take the first occurrence of
	// each normalised key). To support both orderings, we collect (date, label) pairs
	// then sort by date descending before deduplicating — this guarantees correctness
	// regardless of the input order.

	type entry struct {
		timeUnix int64
		label    string
	}
	raw := make([]entry, 0, len(txns))
	for _, t := range txns {
		label := strings.TrimSpace(t.Payee)
		if label == "" {
			label = strings.TrimSpace(t.Desc)
		}
		if label == "" {
			continue
		}
		raw = append(raw, entry{t.Date.Unix(), label})
	}

	// Sort descending by time (simple insertion-style — input is often already
	// near-sorted, so this is fast in practice; worst-case O(n²) is fine for
	// payee lists which are small).
	for i := 1; i < len(raw); i++ {
		for j := i; j > 0 && raw[j].timeUnix > raw[j-1].timeUnix; j-- {
			raw[j], raw[j-1] = raw[j-1], raw[j]
		}
	}

	seen := make(map[string]struct{})
	out := make([]string, 0, min(len(raw), max(limit, 0)))
	for _, e := range raw {
		key := strings.ToLower(e.label)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, e.label)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
