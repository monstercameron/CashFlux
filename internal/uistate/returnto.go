// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v5/state"

// ReturnTo is a one-hop breadcrumb: where a cross-page action came FROM, what the
// user was doing there, and where it was going.
//
// C581: correcting a transaction legitimately sends you to Rules, Categories,
// Activity or the To-do list — but the ledger's filter, page and scroll position
// are exactly the context you need back afterwards, and the app's answer was the
// browser Back button and hope. Worse, the nav rail highlights the destination, so
// the trip reads as "you have left the transaction task" rather than "you have
// stepped out of it for a second".
//
// Nothing here restores state by replaying it: the ledger's filter is already
// persisted, so returning to the route restores the working set on its own. What
// was missing was a visible, NAMED way back — a control that says which task it
// returns you to, so the user can see the thread they are still holding.
type ReturnTo struct {
	// From is the route to return to, To is the route this breadcrumb is valid on.
	// Storing both is what keeps the crumb from following the user around: it shows
	// on To and nowhere else, and is dropped the moment they navigate somewhere
	// that is neither.
	From string
	To   string
	// Label names the task in the user's terms — "Transactions, filtered to
	// \"coffee\"" — because "Back" alone does not tell them what they are going back
	// to, which is the whole complaint.
	Label string
	// TxnID is the row the trip started from, restored on return (C605). The filter
	// brings the right LIST back; it does not bring back your place in it, and on a
	// list of 122 rows that is most of what "where I was" means. Empty when the trip
	// did not start from a particular row (the toolbar's Rules button).
	TxnID string
	// ReopenReview reopens the Review surface on return (C559). A trip that starts
	// INSIDE Review — "this charge needs a category that does not exist yet" — ends
	// on the ledger with the queue closed unless someone says otherwise, which is
	// the same stranding the crumb exists to prevent, one level in. The card and
	// scope come back with it because ReviewSession outlives the surface.
	ReopenReview bool
}

const returnToAtomID = "nav:returnTo"

// UseReturnTo returns the shared breadcrumb atom.
func UseReturnTo() state.Atom[ReturnTo] {
	a := state.UseAtom(returnToAtomID, ReturnTo{})
	capturedReturnTo = a
	returnToCaptured = true
	return a
}

var (
	capturedReturnTo state.Atom[ReturnTo]
	returnToCaptured bool
)

// SetReturnTo records the breadcrumb from outside a component render — from the
// click handler that is about to navigate away.
func SetReturnTo(r ReturnTo) {
	if returnToCaptured {
		capturedReturnTo.Set(r)
	}
}

// ClearReturnTo drops the breadcrumb.
func ClearReturnTo() {
	if returnToCaptured {
		capturedReturnTo.Set(ReturnTo{})
	}
}
