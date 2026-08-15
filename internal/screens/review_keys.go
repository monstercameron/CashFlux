// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

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
	a := currentReviewKeys
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
		return false
	}
	fn()
	return true
}

// ClearReviewKeys drops the actions when the surface closes, so a stray key
// cannot drive a surface that is no longer on screen.
func ClearReviewKeys() { currentReviewKeys = reviewKeyActions{} }
