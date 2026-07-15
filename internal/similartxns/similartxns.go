// SPDX-License-Identifier: MIT

// Package similartxns finds transactions that "look like" a given one, so the
// ledger can offer to recategorize them together after a single correction (TX7).
//
// Similarity is decided by a stable match key (see keyOf), computed primarily
// from the cleaned payee name (TX1: a learned alias or the normalizer rule pack
// unifies "AMZN Mktp*..." and "Amazon" onto one key). When a transaction has no
// usable payee/description, an optional rules-engine fallback groups it by the
// first rule that matches, so rule-driven merchants still cluster.
//
// It has no syscall/js dependency and unit-tests on native Go.
package similartxns

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/payeealias"
	"github.com/monstercameron/CashFlux/internal/rules"
)

// Candidate is a transaction that matches the target and is a recategorize
// candidate because its category differs from (or is absent versus) the target's.
type Candidate struct {
	// Txn is the matching transaction (raw, unmodified).
	Txn domain.Transaction
	// AlreadyCategorized reports whether the candidate already has some category
	// (a different one). The UI lists these but must never overwrite them without
	// an explicit click (respecting the already-categorized semantics).
	AlreadyCategorized bool
}

// keysOf returns a transaction's similarity keys. payeeKey is "p:<clean-name>"
// from the resolved payee/description (TX1 primary path); ruleKey is
// "r:<ruleID>" from the first matching rule (secondary path). Either may be ""
// when that path yields nothing. Two transactions are similar when they share a
// non-empty key on EITHER path.
func keysOf(t domain.Transaction, resolver *payeealias.Resolver, rs []rules.Rule) (payeeKey, ruleKey string) {
	label := strings.TrimSpace(t.Payee)
	if label == "" {
		label = strings.TrimSpace(t.Desc)
	}
	if label != "" {
		if clean := resolver.Resolve(label); clean != "" {
			payeeKey = "p:" + strings.ToLower(clean)
		}
	}
	if len(rs) > 0 {
		if r := rules.FirstMatch(rs, label); r != nil {
			ruleKey = "r:" + r.ID
		}
	}
	return payeeKey, ruleKey
}

// Find returns the transactions similar to target that are recategorize
// candidates for newCategoryID: they share target's match key but currently carry
// a DIFFERENT category (or none). Transactions already set to newCategoryID are
// skipped (nothing to change), as is the target itself. Input order is preserved.
//
// resolver may be nil (the rule pack still applies). rs may be nil (payee/alias
// matching only). When target has no usable key the result is empty.
func Find(target domain.Transaction, all []domain.Transaction, newCategoryID string, resolver *payeealias.Resolver, rs []rules.Rule) []Candidate {
	tPayee, tRule := keysOf(target, resolver, rs)
	if tPayee == "" && tRule == "" {
		return nil
	}
	var out []Candidate
	for _, t := range all {
		if t.ID == target.ID {
			continue
		}
		if t.CategoryID == newCategoryID {
			continue // already where we'd put it — nothing to offer
		}
		p, r := keysOf(t, resolver, rs)
		similar := (tPayee != "" && p == tPayee) || (tRule != "" && r == tRule)
		if !similar {
			continue
		}
		out = append(out, Candidate{
			Txn:                t,
			AlreadyCategorized: strings.TrimSpace(t.CategoryID) != "",
		})
	}
	return out
}
