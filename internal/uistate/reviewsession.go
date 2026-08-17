// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"github.com/monstercameron/CashFlux/internal/reviewscope"
	"github.com/monstercameron/GoWebComponents/v5/state"
)

// ReviewSession is the review surface's place in its own work: which population
// it is working through, how much one confirmation writes, which mode it is in,
// and which card it is on.
//
// It lives in an atom rather than in the surface's component state because two
// tickets need it to outlive the surface being closed:
//
//   - C559 — correcting a missing category or turning a repeated correction into a
//     rule means leaving for /categories or /rules. Coming back has to land on the
//     same card in the same scope, or the "return" control returns you to a
//     different job than the one you left.
//   - C554 — the scope is a deliberate choice. Re-deriving it on every open would
//     silently undo the choice each time the surface is dismissed.
//
// ScopeChosen is what tells the surface "nobody has decided, follow the ledger"
// apart from "the user has decided, leave it alone".
type ReviewSession struct {
	ScopeChosen bool
	Scope       reviewscope.Scope
	Commit      reviewscope.CommitScope
	// CommitKey is the merchant the commit scope was chosen FOR. A scope that
	// outlived the card it was chosen on is how "Categorize all 122" would end up
	// aimed at a merchant with three charges the user never looked at, so the
	// surface only honours Commit while CommitKey still matches the card (C653).
	CommitKey string
	Mode      string
	Cursor    int
	// CursorKey is the merchant the card is ON, and it outranks Cursor when the
	// merchant is still queued.
	//
	// An index alone is not a place in this list. reviewqueue.GroupByMerchant ranks
	// groups by (confidence, size, key), so categorizing ONE charge shrinks its
	// group and can drop it below a same-confidence neighbour — and a cursor that
	// stayed at index 3 would then be looking at a different merchant it had never
	// shown the user. Holding the key means "stay on this merchant until it is
	// cleared", which is what "and next" has to mean for a per-charge commit. When
	// the merchant IS cleared the key stops matching and Cursor takes over, which
	// is correct: the work at that position is now the next merchant.
	CursorKey string
}

// Normalized returns s with every field readable, so a stale persisted value can
// never leave the surface in a state it has no rendering for.
func (s ReviewSession) Normalized() ReviewSession {
	s.Scope = reviewscope.Normalize(s.Scope)
	s.Commit = reviewscope.NormalizeCommit(s.Commit)
	if s.Mode != ReviewModeSingle && s.Mode != ReviewModeBulk {
		s.Mode = ReviewModeBulk
	}
	if s.Cursor < 0 {
		s.Cursor = 0
	}
	return s
}

// The review surface's two modes. They live here rather than in the screens
// package because the session that records the current one does.
const (
	ReviewModeSingle = "single"
	ReviewModeBulk   = "bulk"
)

const reviewSessionAtomID = "review:session"

var (
	capturedReviewSession state.Atom[ReviewSession]
	reviewSessionCaptured bool
)

// UseReviewSession returns the shared review-session atom. Calling it in a render
// also captures it for ResetReviewSession.
func UseReviewSession() state.Atom[ReviewSession] {
	a := state.UseAtom(reviewSessionAtomID, ReviewSession{})
	capturedReviewSession = a
	reviewSessionCaptured = true
	return a
}

// ResetReviewSession forgets the session so the next open re-derives its scope
// from the ledger. Called when Review is finished, not when it is merely closed:
// closing the surface to go and create a category is exactly the case the session
// exists to survive.
func ResetReviewSession() {
	if reviewSessionCaptured {
		capturedReviewSession.Set(ReviewSession{})
	}
}
