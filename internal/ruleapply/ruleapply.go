// SPDX-License-Identifier: MIT

// Package ruleapply makes applying a categorization rule to existing
// transactions scoped and REVERSIBLE (WF7).
//
// Most of the rules workbench already exists: match counts, an affected-
// transactions preview, shadowed-rule warnings, drag-to-reorder precedence, and
// a dry run before a bulk apply. What did not exist is the part the code itself
// flagged — appstate's own comment calls apply-to-existing "an overwrite that
// has no per-rule undo".
//
// That is the asymmetry worth fixing. A rule that matches nothing is a mild
// annoyance somebody notices immediately. A rule applied across years of history
// is a quiet mess nobody notices until a report looks wrong months later, and
// until now there was no way back.
//
// Two ideas, both small:
//
//   - A SCOPE, so history and the future are separate decisions. Applying to
//     what arrives later is safe and reversible by deleting the rule; applying
//     to history is neither.
//   - A recorded CHANGE per transaction carrying the old value, so an undo
//     restores what was there rather than clearing the field.
package ruleapply

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/rules"
)

// Scope is which transactions a bulk application touches.
type Scope string

const (
	// ScopeFuture leaves history alone. The safe default, and the one a hesitant
	// user should be able to pick without thinking: history is the part that
	// cannot be un-seen.
	ScopeFuture Scope = "future"
	// ScopeAll applies to every matching transaction ever recorded.
	ScopeAll Scope = "all"
	// ScopeSince applies from a chosen date forward.
	ScopeSince Scope = "since"
)

// Normalized defaults an unknown scope to the safe one.
//
// Defaulting to ScopeFuture rather than refusing, because the caller here is a
// surface with a button to draw — and if a stored preference is ever unreadable,
// the failure should be "did less than expected", never "rewrote your history".
func (s Scope) Normalized() Scope {
	switch s {
	case ScopeAll, ScopeSince:
		return s
	default:
		return ScopeFuture
	}
}

// InScope reports whether a transaction falls inside an application's scope.
func InScope(t domain.Transaction, scope Scope, since time.Time) bool {
	switch scope.Normalized() {
	case ScopeAll:
		return true
	case ScopeSince:
		return !since.IsZero() && !t.Date.Before(since)
	default:
		return false
	}
}

// Change is one edit an application made, carrying enough to reverse it.
type Change struct {
	TxnID string
	// FromCategoryID is what the transaction was categorized as BEFORE.
	//
	// Storing the old value rather than a flag is what makes undo honest: a
	// transaction that was already categorized has to go back to that category,
	// not to blank, and without this the two are indistinguishable.
	FromCategoryID string
	ToCategoryID   string
}

// Plan lists the edits applying a rule would make within a scope.
//
// Only rows that actually CHANGE are returned. Setting a category to the one it
// already has is not an edit, and counting it would inflate both the warning
// beforehand and the undo afterwards.
//
// Ordered by transaction ID so the same rule against the same ledger always
// produces the same plan — a bulk edit that shuffles between runs cannot be
// diffed, reviewed, or trusted.
func Plan(r rules.Rule, txns []domain.Transaction, scope Scope, since time.Time) []Change {
	if strings.TrimSpace(r.SetCategoryID) == "" {
		// A rule with no category to set has nothing to apply here. Its other
		// actions (tags, rename, member) are apply-once by design and are not this
		// package's business.
		return nil
	}
	var out []Change
	for _, t := range txns {
		if !InScope(t, scope, since) {
			continue
		}
		if !r.MatchesTxn(ctxOf(t)) || t.CategoryID == r.SetCategoryID {
			continue
		}
		out = append(out, Change{TxnID: t.ID, FromCategoryID: t.CategoryID, ToCategoryID: r.SetCategoryID})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].TxnID < out[j].TxnID })
	return out
}

// Undo inverts a recorded set of changes.
//
// It takes the RECORD rather than re-deriving from the rule, and that is the
// whole point: by the time somebody wants to undo, the rule may have been
// edited or deleted, and re-running it would restore something else entirely.
// What was captured at the time is the only trustworthy account of what to put
// back.
func Undo(changes []Change) []Change {
	out := make([]Change, 0, len(changes))
	for _, c := range changes {
		out = append(out, Change{TxnID: c.TxnID, FromCategoryID: c.ToCategoryID, ToCategoryID: c.FromCategoryID})
	}
	return out
}

// Reclassified counts changes that overwrote an EXISTING category, as distinct
// from ones that merely filled a blank.
//
// The number that should give somebody pause. Filing 200 uncategorized
// transactions is the rule doing its job; overwriting 200 categories a person
// chose by hand is a different act, and one figure covering both hides it.
func Reclassified(changes []Change) int {
	n := 0
	for _, c := range changes {
		if c.FromCategoryID != "" {
			n++
		}
	}
	return n
}

// ManyCategories is how many distinct existing categories a rule's changes may
// span before it looks over-broad.
//
// Three. Two is ordinary — a payee moved category once. Three separate existing
// meanings is a pattern that is not describing one thing.
const ManyCategories = 3

// OverBroad reports whether the changes span enough different existing
// categories to suggest the rule is catching things it should not.
//
// Blank categories are not counted: uncategorized rows say nothing about what a
// rule means, and letting them push the count up would flag every genuinely
// useful backfill.
func OverBroad(changes []Change) (int, bool) {
	seen := map[string]bool{}
	for _, c := range changes {
		if c.FromCategoryID != "" {
			seen[c.FromCategoryID] = true
		}
	}
	return len(seen), len(seen) >= ManyCategories
}

// ctxOf converts a transaction into the shape the rules engine matches on.
func ctxOf(t domain.Transaction) rules.TxnCtx {
	return rules.TxnCtx{
		Payee:       t.Payee,
		Desc:        t.Desc,
		AmountMinor: t.Amount.Amount,
		AccountID:   t.AccountID,
		Date:        rules.NewTxnDate(t.Date),
	}
}
