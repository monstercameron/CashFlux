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
// AccountIDs plays two roles (QA CF-01/UX-03): beside a non-empty dimensional
// filter it is an additive union (listed IDs are included even when they miss
// the dimensions); when it is the ONLY non-empty part of the scope it is a
// restriction — the scope is exactly those accounts. Archived accounts are
// never returned either way.
//
// Callers: keep ReportScope values comparable by always leaving unused
// slices nil rather than empty.
type ReportScope struct {
	// Institutions narrows to accounts whose institution name (returned by the
	// institutionOf accessor) matches any of these values, case-insensitively.
	Institutions []string

	// Owners narrows to a member perspective: an account matches when its
	// OwnerID is any of these values OR the account is SHARED (Scope ==
	// domain.ScopeShared, or OwnerID == domain.GroupOwnerID). This mirrors the
	// app-wide member-visibility rule ("mine + the household's") — the account
	// data model marks sharing via Account.Scope while OwnerID records the
	// creating member, so exact-owner matching would wrongly hide joint
	// accounts from every member who didn't create them.
	Owners []string

	// Types narrows to accounts whose Type matches any of these values.
	Types []domain.AccountType

	// AccountIDs: additive beside a dimensional filter; the exact scope when it
	// is the only non-empty part (see ResolveScope rule 4, QA CF-01/UX-03).
	// Always subject to the archived exclusion.
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
//  3. When any dimensional filter (Institutions, Owners, Types) is non-empty,
//     an account qualifies dimensionally when it satisfies every non-empty
//     dimension simultaneously; within each dimension membership is OR-tested.
//     Any ID listed in s.AccountIDs is then unioned in regardless of whether
//     it satisfied the dimensions (AccountIDs stays additive beside dimensions).
//  4. When AccountIDs is the ONLY non-empty part of the scope, it is a
//     RESTRICTION, not an addition: the result is exactly those live accounts.
//     (The old behavior ran the dimensional loop with no dimensions — which
//     matches everything — so "Specific accounts: Regression Checking" showed
//     "Scope (1)" while every figure stayed household-wide: QA CF-01/UX-03.)
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
	dimensional := len(s.Institutions) > 0 || len(s.Owners) > 0 || len(s.Types) > 0

	switch {
	case s.IsAll():
		// All live accounts are in scope.
		for id := range live {
			seen[id] = struct{}{}
		}
	default:
		if dimensional {
			// Dimensional filter: AND across non-empty dimensions.
			for _, a := range live {
				if matchesDimensions(a, s, institutionOf) {
					seen[a.ID] = struct{}{}
				}
			}
		}
		// AccountIDs: additive beside a dimensional filter; the whole scope
		// when it is the only thing selected.
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

	// Owner dimension: a member's perspective includes the accounts they own AND
	// the household's shared accounts (Scope=shared, or the group pseudo-owner) —
	// the same "mine + shared" rule the rest of the app applies to member scoping.
	if len(s.Owners) > 0 {
		shared := a.Scope == domain.ScopeShared || a.OwnerID == domain.GroupOwnerID
		if !shared && !anyStringExact(a.OwnerID, s.Owners) {
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

// Merge combines the app-wide viewing lens (the top-bar "Viewing as" scope)
// with a report-local scope into the effective scope for a report view.
//
// Rule: dimension-wise, the local scope wins wherever it says anything; a
// dimension the local scope leaves empty falls back to the lens. This keeps a
// report narrowed inside whatever the household is currently "viewing as"
// while guaranteeing a filter chosen on the report page never rewrites the
// app-wide lens (the commercial-parity scan's "report scope leaks globally"
// defect).
func Merge(lens, local ReportScope) ReportScope {
	out := local
	if len(out.Owners) == 0 {
		out.Owners = lens.Owners
	}
	if len(out.Institutions) == 0 {
		out.Institutions = lens.Institutions
	}
	if len(out.Types) == 0 {
		out.Types = lens.Types
	}
	if len(out.AccountIDs) == 0 {
		out.AccountIDs = lens.AccountIDs
	}
	return out
}
