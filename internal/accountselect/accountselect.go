// SPDX-License-Identifier: MIT

// Package accountselect chooses the account to pre-fill in the Quick-Add
// transaction form (this file) and prepares the account-transfer form
// (transfer.go). The selection helpers use only internal/domain and the standard
// library; the transfer helpers also use appstate (for TransferParams), goalview,
// acctsort, currency, money and dateutil. It contains no syscall/js, so it
// compiles and unit-tests on native Go.
package accountselect

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// recentWindow is the look-back period used by the frequency tier.
const recentWindow = 90 * 24 * time.Hour

// isSpendAccount reports whether an account qualifies as a "spend from" default:
// it must be a non-archived asset account whose type is not a valuation holding
// (investment, retirement, crypto, property, vehicle or other). Those types are
// not natural places to record everyday spending transactions.
func isSpendAccount(a domain.Account) bool {
	return !a.Archived &&
		a.Class == domain.ClassAsset &&
		!a.Type.IsValuation()
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
//   - Tier 2 — most-used recent spend account: the non-archived, non-valuation
//     asset account with the most transactions in the 90-day window ending at the
//     most-recent transaction date among the spend-eligible accounts' transactions
//     (not wall-clock time). Anchoring on the data instead of time.Now() keeps the
//     function deterministic and side-effect-free. Ties are broken by account ID
//     (lexicographic ascending) for stability. Use DefaultIDAt to ignore future-dated transactions when anchoring.
//
//   - Tier 3 — first checking/debit/savings asset: the first non-archived
//     checking, debit, or savings account in the order given.
//
//   - Tier 4 — first non-valuation asset: the first non-archived asset account
//     whose type is not a valuation holding, in the order given.
//
//   - Tier 5 — none: "" is returned when no account qualifies.
func DefaultID(accounts []domain.Account, txns []domain.Transaction, memberDefaultID string) string {
	return DefaultIDAt(accounts, txns, memberDefaultID, time.Time{})
}

// DefaultIDAt is DefaultID with the recency window anchored on the latest
// eligible transaction dated on or before now, so a future-dated or scheduled
// row cannot push the window past the real activity. A zero now applies no
// upper bound, which is DefaultID.
func DefaultIDAt(accounts []domain.Account, txns []domain.Transaction, memberDefaultID string, now time.Time) string {
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
		// Anchor the recency window on the latest eligible transaction, ignoring
		// dates after now when now is set.
		var maxDate time.Time
		anchored := false
		for _, t := range txns {
			if _, ok := eligible[t.AccountID]; !ok || (!now.IsZero() && t.Date.After(now)) {
				continue
			}
			if !anchored || t.Date.After(maxDate) {
				maxDate, anchored = t.Date, true
			}
		}
		cutoff := maxDate.Add(-recentWindow)

		// Count transactions per eligible account within the window.
		freq := make(map[string]int, len(eligible))
		for _, t := range txns {
			if _, ok := eligible[t.AccountID]; !ok {
				continue
			}
			if anchored && !t.Date.Before(cutoff) && (now.IsZero() || !t.Date.After(now)) {
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
			if c > bestCount || (c == bestCount && bestID == "") {
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

	// --- Tier 4: first non-valuation asset ---
	for _, a := range accounts {
		if isSpendAccount(a) {
			return a.ID
		}
	}

	// --- Tier 5: none ---
	return ""
}

// Tier identifies which source selected an account for a new transaction.
type Tier string

const (
	// TierExplicit means a valid account explicitly selected for this transaction.
	TierExplicit Tier = "explicit"
	// TierMember means the active member's saved default selected the account.
	TierMember Tier = "member"
	// TierWorkspace means the workspace's saved default selected the account.
	TierWorkspace Tier = "workspace"
	// TierHeuristic means DefaultID selected the account from transaction history
	// or eligible account types.
	TierHeuristic Tier = "heuristic"
	// TierFirstAccount means the first active account was used as the last fallback.
	TierFirstAccount Tier = "first-account"
	// TierNone means no non-archived account was available.
	TierNone Tier = "none"
)

// ResolveInput contains the persisted and contextual values used to select an
// account for a new transaction.
type ResolveInput struct {
	Accounts           []domain.Account
	Transactions       []domain.Transaction
	ExplicitID         string
	ActiveMemberID     string
	Members            []domain.Member
	WorkspaceDefaultID string
	// Now bounds the heuristic's recency window: transactions dated after it are
	// ignored. Zero means no bound.
	Now time.Time
}

// Resolve selects an account for a new transaction using explicit, member,
// workspace, heuristic, then first-account precedence. Saved defaults and
// explicit choices are honored only when they identify an existing, non-archived
// account. Invalid defaults are skipped without changing their stored values.
// The heuristic is deterministic and uses only the supplied account and
// transaction data; it does not consult wall-clock or device state.
func Resolve(input ResolveInput) (string, Tier) {
	if validAccountID(input.Accounts, input.ExplicitID) {
		return input.ExplicitID, TierExplicit
	}

	if input.ActiveMemberID != "" {
		for _, member := range input.Members {
			if member.ID == input.ActiveMemberID && validAccountID(input.Accounts, member.Prefs.DefaultAccountID) {
				return member.Prefs.DefaultAccountID, TierMember
			}
		}
	}

	if validAccountID(input.Accounts, input.WorkspaceDefaultID) {
		return input.WorkspaceDefaultID, TierWorkspace
	}

	if id := DefaultIDAt(input.Accounts, input.Transactions, "", input.Now); id != "" {
		return id, TierHeuristic
	}

	for _, account := range input.Accounts {
		if account.ID != "" && !account.Archived {
			return account.ID, TierFirstAccount
		}
	}

	return "", TierNone
}

func validAccountID(accounts []domain.Account, id string) bool {
	if id == "" {
		return false
	}
	for _, account := range accounts {
		if account.ID == id && account.ID != "" && !account.Archived {
			return true
		}
	}
	return false
}
