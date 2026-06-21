//go:build js && wasm

package app

import (
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// How long a notice stays on screen before auto-dismissing. Errors linger longer
// than ordinary notices so there's time to read what went wrong.
const (
	toastTimeoutMS    = 4500
	toastErrTimeoutMS = 7500
)

// Toast renders the app-wide notice surface: a single dismissible message
// pinned to the bottom of the viewport, driven by the shared uistate.Notice
// atom. Ordinary notices auto-dismiss after toastTimeoutMS; errors linger for
// toastErrTimeoutMS (and can always be dismissed by hand).
func Toast() uic.Node {
	atom := uistate.UseNotice()
	n := atom.Get()

	// Wire the app's workflow "notify" action to this toast surface, once. The
	// captured atom lets a notice posted from event time (a workflow run) show up
	// here without calling a hook outside render.
	uic.UseEffect(func() func() {
		if appstate.Default != nil {
			appstate.Default.Notifier = func(msg string) {
				atom.Set(atom.Get().With(msg, false))
			}
		}
		return nil
	}, "wire-notifier")

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
		timeout := toastTimeoutMS
		if n.Err {
			timeout = toastErrTimeoutMS
		}
		id := js.Global().Call("setTimeout", cb, timeout)
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
		return Div(ClassStr("sr-only"), Attr("role", role), Attr("aria-live", live))
	}
	cls := "toast"
	if n.Err {
		cls += " toast-err"
	}
	return Div(ClassStr(cls), Attr("role", role), Attr("aria-live", live),
		Span(ClassStr("toast-msg"), n.Text),
		Button(ClassStr("toast-x"), Attr("type", "button"), Attr("title", "Dismiss"), Attr("aria-label", "Dismiss"),
			OnClick(func() { atom.Set(n.Cleared()) }), "×"),
	)
}
