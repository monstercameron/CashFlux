//go:build js && wasm

package app

import (
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// toastTimeoutMS is how long a notice stays on screen before auto-dismissing.
const toastTimeoutMS = 4500

// Toast renders the app-wide notice surface: a single dismissible message
// pinned to the bottom of the viewport, driven by the shared uistate.Notice
// atom. It auto-dismisses after toastTimeoutMS; errors persist styling but
// still time out (the action that caused them has already failed safely).
func Toast() uic.Node {
	atom := uistate.UseNotice()
	n := atom.Get()

	// Re-arm the auto-dismiss timer whenever a new notice is posted (keyed on
	// Seq). The cleanup clears a still-pending timer and releases its callback
	// so switching notices quickly never leaks funcs or fires a stale clear.
	uic.UseEffect(func() func() {
		if n.Text == "" {
			return nil
		}
		cb := js.FuncOf(func(js.Value, []js.Value) any {
			atom.Set(n.Cleared()) // keeps Seq, so this effect does not re-run
			return nil
		})
		id := js.Global().Call("setTimeout", cb, toastTimeoutMS)
		// Cleanup runs only on the next post (Seq change) or unmount — by then
		// the timer has either fired or is pending; clear it and release once.
		return func() {
			js.Global().Call("clearTimeout", id)
			cb.Release()
		}
	}, n.Seq)

	// Errors interrupt (assertive/alert); ordinary notices are announced politely.
	// Keeping the same live-region element mounted across the empty and active
	// states — rather than mounting a fresh node only when there's text — is what
	// makes screen readers reliably announce each new notice.
	live, role := "polite", "status"
	if n.Err {
		live, role = "assertive", "alert"
	}
	if n.Text == "" {
		// Idle: an empty, visually-hidden live region stays in the DOM so the next
		// post is announced (a region inserted together with its text often isn't).
		return Div(Class("sr-only"), Attr("role", role), Attr("aria-live", live))
	}
	cls := "toast"
	if n.Err {
		cls += " toast-err"
	}
	return Div(Class(cls), Attr("role", role), Attr("aria-live", live),
		Span(Class("toast-msg"), n.Text),
		Button(Class("toast-x"), Attr("type", "button"), Attr("title", "Dismiss"),
			OnClick(func() { atom.Set(n.Cleared()) }), "×"),
	)
}
