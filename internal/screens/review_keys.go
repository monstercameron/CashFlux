// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import "syscall/js"

// reviewKeyActions is what the surface exposes to the keyboard. A nil field
// means the key does nothing in the current mode.
type reviewKeyActions struct {
	Next    func() // j / ArrowDown
	Prev    func() // k / ArrowUp
	Confirm func() // Enter
	Skip    func() // s
	Dismiss func() // d
	Toggle  func() // space — select the focused merchant (bulk)
	Bulk    func() // b
	Single  func() // 1
}

// currentReviewKeys is what the live listener dispatches to. The surface has a
// single instance, and the listener must be registered from an UNCONDITIONAL
// hook (hook order is positional in this framework), so the render assigns the
// actions here each pass instead of re-registering a fresh closure.
var currentReviewKeys reviewKeyActions

// reviewSurfaceMounted reports whether the review surface currently owns the
// review-inbox node. The shell unmounts that surface on every close path, so
// this check also protects against actions left behind by its last render.
func reviewSurfaceMounted() bool {
	doc := js.Global().Get("document")
	if doc.IsNull() || doc.IsUndefined() {
		return false
	}
	node := doc.Call("querySelector", "[data-testid='review-inbox']")
	return !node.IsNull() && !node.IsUndefined()
}

// setReviewKeys publishes this render's actions to the live listener.
func setReviewKeys(a reviewKeyActions) { currentReviewKeys = a }

// HandleReviewKey dispatches one key to the open review surface and reports
// whether it was consumed. Called from the app's single global keydown handler
// (internal/app/shortcuts.go).
//
// It lives there rather than in a UseEffect inside the surface because effects
// do NOT run for a component passed as a FlipPanel prop — the listener silently
// never registered, which is exactly the kind of bug a hint promising "j / k
// move" would have shipped on top of.
//
// The caller has already excluded modifier combos and editable targets.
func HandleReviewKey(key string) bool {
	return dispatchReviewKey(key, currentReviewKeys, reviewSurfaceMounted(), HandleTxnKey)
}

func dispatchReviewKey(key string, a reviewKeyActions, mounted bool, fallback func(string) bool) bool {
	if !mounted {
		return fallback(key)
	}
	var fn func()
	switch key {
	case "j", "ArrowDown":
		fn = a.Next
	case "k", "ArrowUp":
		fn = a.Prev
	case "Enter":
		fn = a.Confirm
	case "s":
		fn = a.Skip
	case "d":
		fn = a.Dismiss
	case " ", "Spacebar":
		fn = a.Toggle
	case "b":
		fn = a.Bulk
	case "1":
		fn = a.Single
	}
	if fn == nil {
		// The app owns one global keydown listener. When the review surface has no
		// action for a key, offer it to the ledger without changing review's map.
		return fallback(key)
	}
	fn()
	return true
}

// ClearReviewKeys drops the actions when the surface closes, so a stray key
// cannot drive a surface that is no longer on screen.
func ClearReviewKeys() { currentReviewKeys = reviewKeyActions{} }
