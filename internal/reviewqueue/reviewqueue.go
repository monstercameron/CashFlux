// SPDX-License-Identifier: MIT

// Package reviewqueue is the pure selection model behind the transaction Review
// inbox (CG-S2) — the guided triage flow that lets a user clear new / imported /
// uncategorized transactions the way YNAB (approve), Copilot, and Monarch
// (review) do. It has no syscall/js: it decides which transactions still need a
// human look and in what order, so it unit-tests on native Go and the UI just
// steps through Queue.
package reviewqueue

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// ReviewTag mirrors workflow.ReviewTag ("needs-review") — the tag auto-review
// workflows add to flag a transaction for a human look. Duplicated as a const so
// this selection package stays dependency-light (importing workflow would pull
// the whole rules engine in).
const ReviewTag = "needs-review"

// Reason explains why a transaction is in the queue, so the UI can label it.
type Reason int

const (
	// ReasonUncategorized: the transaction has no category yet.
	ReasonUncategorized Reason = iota
	// ReasonFlagged: the transaction is categorized but tagged for review.
	ReasonFlagged
)

func hasReviewTag(t domain.Transaction) bool {
	for _, tag := range t.Tags {
		if tag == ReviewTag {
			return true
		}
	}
	return false
}

// Needs reports whether a transaction should appear in the review queue: a
// non-transfer that is either uncategorized or explicitly flagged for review.
// Transfers move money between the user's own accounts and don't need a spend
// category, so they're never queued.
func Needs(t domain.Transaction) bool {
	if t.IsTransfer() {
		return false
	}
	// C492: splits win over CategoryID in every analytics path, so a split that
	// adds up is fully categorized no matter what CategoryID says — queueing it
	// would invite a flat assignment that changes nothing. A split that does NOT
	// add up is genuinely unfinished and stays queued, with its own reason.
	if t.HasSplits() {
		return !t.SplitsReconcile()
	}
	return t.CategoryID == "" || hasReviewTag(t)
}

// ReasonFor returns why a transaction is queued. An unbalanced split outranks
// the others: assigning a category would not fix it, so the UI must not offer
// that as the primary action. Otherwise being uncategorized takes precedence
// over a review flag.
func ReasonFor(t domain.Transaction) Reason {
	if t.HasSplits() && !t.SplitsReconcile() {
		return ReasonSplitUnbalanced
	}
	if t.CategoryID == "" {
		return ReasonUncategorized
	}
	return ReasonFlagged
}

// QueueOpen returns the transactions needing review that the user has not
// snoozed or dismissed (C493), newest first. Pass a nil Resolutions to get the
// unfiltered queue.
func QueueOpen(txns []domain.Transaction, rs Resolutions, now time.Time) []domain.Transaction {
	out := make([]domain.Transaction, 0)
	for _, t := range txns {
		if Needs(t) && !rs.Suppressed(t.ID, now) {
			out = append(out, t)
		}
	}
	return sortQueue(out)
}

// CountOpen is QueueOpen's length — the number the inbox badge shows once
// snoozed and dismissed charges stop counting against the user.
func CountOpen(txns []domain.Transaction, rs Resolutions, now time.Time) int {
	n := 0
	for _, t := range txns {
		if Needs(t) && !rs.Suppressed(t.ID, now) {
			n++
		}
	}
	return n
}

// Queue returns the transactions needing review, newest first (ties broken by id
// for a stable order), so a fresh import surfaces at the top. The input is not
// modified.
func Queue(txns []domain.Transaction) []domain.Transaction {
	out := make([]domain.Transaction, 0)
	for _, t := range txns {
		if Needs(t) {
			out = append(out, t)
		}
	}
	return sortQueue(out)
}

// sortQueue orders newest-first with a stable id tiebreak.
func sortQueue(out []domain.Transaction) []domain.Transaction {
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].Date.Equal(out[j].Date) {
			return out[i].Date.After(out[j].Date)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Count returns how many transactions need review — the number the inbox badge
// shows.
func Count(txns []domain.Transaction) int {
	n := 0
	for _, t := range txns {
		if Needs(t) {
			n++
		}
	}
	return n
}
