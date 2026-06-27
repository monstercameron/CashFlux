// SPDX-License-Identifier: MIT

// Package accountselect provides a pure-Go helper for choosing the best default
// account to pre-fill in the Quick-Add transaction form. It is dependency-light
// (only internal/domain and stdlib) and contains no syscall/js, so it compiles
// and unit-tests on native Go.
package accountselect

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// recentWindow is the look-back period used by the frequency tier.
const recentWindow = 90 * 24 * time.Hour

// isSpendAccount reports whether an account qualifies as a "spend from" default:
// it must be a non-archived asset account that is not a non-spending asset type
// (investment, retirement, or crypto). Those types are not natural places to
// record everyday spending transactions.
func isSpendAccount(a domain.Account) bool {
	return !a.Archived &&
		a.Class == domain.ClassAsset &&
		a.Type != domain.TypeInvestment &&
		a.Type != domain.TypeRetirement &&
		a.Type != domain.TypeCrypto
}

// isCheckingLike reports whether a is a checking, debit, or savings account —
// the canonical everyday-spending types used by tier 3.
func isCheckingLike(a domain.Account) bool {
	return a.Type == domain.TypeChecking ||
		a.Type == domain.TypeDebit ||
		a.Type == domain.TypeSavings
}

// DefaultID selects the best default account ID for the Quick-Add form using
// the following priority chain. Each tier is tried in order; the first match wins.
//
//   - Tier 1 — explicit member preference: if memberDefaultID is non-empty and
//     names a non-archived account in the accounts slice, that ID is returned
//     immediately. Archived accounts are not honoured (the member's pref is stale).
//
//   - Tier 2 — most-used recent spend account: the non-archived, non-investment
//     asset account with the most transactions in the 90-day window ending at the
//     most-recent transaction date in the txns slice (not wall-clock time). Using
//     the max transaction date instead of time.Now() keeps the function
//     deterministic and side-effect-free — identical inputs always produce the same
//     output regardless of when the call is made. Ties are broken by account ID
//     (lexicographic ascending) for stability.
//
//   - Tier 3 — first checking/debit/savings asset: the first non-archived
//     checking, debit, or savings account in the order given.
//
//   - Tier 4 — first non-investment asset: the first non-archived, non-investment
//     asset account in the order given.
//
//   - Tier 5 — none: "" is returned when no account qualifies.
func DefaultID(accounts []domain.Account, txns []domain.Transaction, memberDefaultID string) string {
	// --- Tier 1: explicit member preference ---
	if memberDefaultID != "" {
		for _, a := range accounts {
			if a.ID == memberDefaultID && !a.Archived {
				return a.ID
			}
		}
	}

	// Build an index of spend-eligible accounts for O(1) lookup in tier 2.
	eligible := make(map[string]struct{}, len(accounts))
	for _, a := range accounts {
		if isSpendAccount(a) {
			eligible[a.ID] = struct{}{}
		}
	}

	// --- Tier 2: most-used recent spend account ---
	if len(txns) > 0 && len(eligible) > 0 {
		// Find the latest transaction date to anchor the recency window.
		maxDate := txns[0].Date
		for _, t := range txns[1:] {
			if t.Date.After(maxDate) {
				maxDate = t.Date
			}
		}
		cutoff := maxDate.Add(-recentWindow)

		// Count transactions per eligible account within the window.
		freq := make(map[string]int, len(eligible))
		for _, t := range txns {
			if _, ok := eligible[t.AccountID]; !ok {
				continue
			}
			if !t.Date.Before(cutoff) {
				freq[t.AccountID]++
			}
		}

		// Find the account with the highest frequency; break ties by ID.
		bestID := ""
		bestCount := 0
		for _, a := range accounts {
			if _, ok := eligible[a.ID]; !ok {
				continue
			}
			c := freq[a.ID]
			if c > bestCount || (c == bestCount && bestID == "" ) {
				bestCount = c
				bestID = a.ID
			} else if c == bestCount && a.ID < bestID {
				bestID = a.ID
			}
		}
		if bestCount > 0 {
			return bestID
		}
	}

	// --- Tier 3: first checking/debit/savings asset ---
	for _, a := range accounts {
		if isSpendAccount(a) && isCheckingLike(a) {
			return a.ID
		}
	}

	// --- Tier 4: first non-investment asset ---
	for _, a := range accounts {
		if isSpendAccount(a) {
			return a.ID
		}
	}

	// --- Tier 5: none ---
	return ""
}
