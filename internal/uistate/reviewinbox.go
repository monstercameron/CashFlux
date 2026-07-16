// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v4/state"

// capturedReviewInbox lets the transactions toolbar's "Review" button open the
// review-inbox flip modal from a click handler without calling UseAtom outside a
// render. Mirrors the payee-clean / task-edit shell-root modal seams.
var (
	capturedReviewInbox state.Atom[bool]
	reviewInboxCaptured bool
)

// UseReviewInbox returns the atom holding whether the transaction Review inbox
// (CG-S2) is open. ReviewInboxHost (shell root) reads it and renders the flip
// modal; the toolbar button sets it. Calling it in a render also captures the
// atom for OpenReviewInbox / CloseReviewInbox.
func UseReviewInbox() state.Atom[bool] {
	a := state.UseAtom("txn:reviewInbox", false)
	capturedReviewInbox = a
	reviewInboxCaptured = true
	return a
}

// OpenReviewInbox opens the review inbox.
func OpenReviewInbox() {
	if reviewInboxCaptured {
		capturedReviewInbox.Set(true)
	}
}

// CloseReviewInbox closes the review inbox.
func CloseReviewInbox() {
	if reviewInboxCaptured {
		capturedReviewInbox.Set(false)
	}
}
