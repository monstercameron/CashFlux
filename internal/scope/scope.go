// SPDX-License-Identifier: MIT

// Package scope implements the pure scope-resolution engine for CashFlux
// multi-institution analytics (task #443). It is entirely platform-independent
// (no syscall/js) and safe to unit-test with native Go.
//
// A ReportScope describes which accounts — and therefore which transactions —
// belong to a particular report view. Dimensions are AND-combined across the
// non-empty ones; within a single dimension the values are OR-combined.
// AccountIDs is always additive: any explicitly listed ID is included even if
// the account would not satisfy the other dimension filters.
//
// The institutionOf accessor is accepted as a parameter so this package compiles
// before the Account.Institution field exists on the domain type. Once that
// field lands, callers may pass:
//
//	func(a domain.Account) string { return a.Institution }
//
// Until then, pass a function that returns "" (or any stub) — Institution
// filtering will simply match all accounts against the empty string.
package scope

import (
	"slices"
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// ReportScope describes the set of accounts a report should cover.
//
// Every dimension is optional; an empty (nil or zero-length) slice means
// "no restriction on this dimension" (i.e., match all). Matching is AND
// across non-empty dimensions, OR within a dimension.
//
// AccountIDs is an additive union: an account whose ID appears in this list
// is always included, regardless of whether it satisfies the other dimensions.
// The union is applied after the dimensional filter, so an archived account
// referenced by ID is still excluded (archived accounts are never returned).
//
// Callers: keep ReportScope values comparable by always leaving unused
// slices nil rather than empty.
type ReportScope struct {
	// Institutions narrows to accounts whose institution name (returned by the
	// institutionOf accessor) matches any of these values, case-insensitively.
	Institutions []string

	// Owners narrows to accounts whose OwnerID matches any of these values.
	// Use domain.GroupOwnerID ("group") to include shared accounts.
	Owners []string

	// Types narrows to accounts whose Type matches any of these values.
	Types []domain.AccountType

	// AccountIDs is an additive override: these account IDs are always
	// included in the resolved set (still subject to the archived exclusion).
	AccountIDs []string
}

// IsAll reports whether every dimension of s is empty, meaning the scope
// covers all non-archived accounts without restriction.
func (s ReportScope) IsAll() bool {
	return len(s.Institutions) == 0 &&
		len(s.Owners) == 0 &&
		len(s.Types) == 0 &&
		len(s.AccountIDs) == 0
}

// ResolveScope returns the sorted, deduplicated set of account IDs that fall
// within scope s, given the full account roster and an institution accessor.
//
// Rules (applied in this order):
//  1. Archived accounts are never returned.
//  2. If IsAll(), every non-archived account is in scope.
//  3. Otherwise an account qualifies dimensionally when it satisfies every
//     non-empty dimension (Institutions, Owners, Types) simultaneously.
//     Within each dimension membership is OR-tested.
//  4. Any ID listed in s.AccountIDs that belongs to a non-archived account is
//     unioned into the result regardless of whether that account satisfied step 3.
//
// institutionOf must not be nil; pass func(domain.Account) string { return "" }
// when no institution data is available (Institutions filter will then match
// only scopes whose Institutions list contains "").
func ResolveScope(
	accounts []domain.Account,
	s ReportScope,
	institutionOf func(domain.Account) string,
) []string {
	// Build a fast lookup: id → account (only non-archived).
	live := make(map[string]domain.Account, len(accounts))
	for _, a := range accounts {
		if !a.Archived {
			live[a.ID] = a
		}
	}

	seen := make(map[string]struct{}, len(live))

	if s.IsAll() {
		// All live accounts are in scope.
		for id := range live {
			seen[id] = struct{}{}
		}
	} else {
		// Dimensional filter: AND across non-empty dimensions.
		for _, a := range live {
			if matchesDimensions(a, s, institutionOf) {
				seen[a.ID] = struct{}{}
			}
		}

		// AccountIDs union: add any explicitly listed live accounts.
		for _, id := range s.AccountIDs {
			if _, ok := live[id]; ok {
				seen[id] = struct{}{}
			}
		}
	}

	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// matchesDimensions reports whether account a satisfies every non-empty
// dimension in s (AND across dimensions, OR within each dimension).
func matchesDimensions(
	a domain.Account,
	s ReportScope,
	institutionOf func(domain.Account) string,
) bool {
	// Institution dimension.
	if len(s.Institutions) > 0 {
		inst := strings.ToLower(institutionOf(a))
		if !anyStringMatch(inst, s.Institutions) {
			return false
		}
	}

	// Owner dimension.
	if len(s.Owners) > 0 {
		if !anyStringExact(a.OwnerID, s.Owners) {
			return false
		}
	}

	// Type dimension.
	if len(s.Types) > 0 {
		if !anyTypeMatch(a.Type, s.Types) {
			return false
		}
	}

	return true
}

// anyStringMatch reports whether target equals (case-insensitively) any value
// in candidates. target is expected to already be lowercased.
func anyStringMatch(target string, candidates []string) bool {
	for _, c := range candidates {
		if strings.ToLower(c) == target {
			return true
		}
	}
	return false
}

// anyStringExact reports whether target exactly equals any value in candidates.
func anyStringExact(target string, candidates []string) bool {
	return slices.Contains(candidates, target)
}

// anyTypeMatch reports whether t equals any type in candidates.
func anyTypeMatch(t domain.AccountType, candidates []domain.AccountType) bool {
	return slices.Contains(candidates, t)
}

// ApplyScopeToTxns returns only the transactions whose AccountID is in the
// provided ids set. Order is preserved.
//
// ids is typically the result of ResolveScope; it must be sorted (as
// ResolveScope guarantees) for the binary-search fast path.
func ApplyScopeToTxns(txns []domain.Transaction, ids []string) []domain.Transaction {
	if len(ids) == 0 {
		return nil
	}
	set := sliceToSet(ids)
	out := make([]domain.Transaction, 0, len(txns))
	for _, t := range txns {
		if _, ok := set[t.AccountID]; ok {
			out = append(out, t)
		}
	}
	return out
}

// ApplyScopeToAccounts returns only the accounts whose ID is in the provided
// ids set. Order is preserved.
//
// ids is typically the result of ResolveScope.
func ApplyScopeToAccounts(accounts []domain.Account, ids []string) []domain.Account {
	if len(ids) == 0 {
		return nil
	}
	set := sliceToSet(ids)
	out := make([]domain.Account, 0, len(accounts))
	for _, a := range accounts {
		if _, ok := set[a.ID]; ok {
			out = append(out, a)
		}
	}
	return out
}

// sliceToSet converts a string slice to a presence map.
func sliceToSet(ss []string) map[string]struct{} {
	m := make(map[string]struct{}, len(ss))
	for _, s := range ss {
		m[s] = struct{}{}
	}
	return m
}
