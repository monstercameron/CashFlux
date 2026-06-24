// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/state"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// How long a notice stays on screen before auto-dismissing. Errors linger longer
// than ordinary notices so there's time to read what went wrong.
const (
	toastTimeoutMS    = 4500
	toastErrTimeoutMS = 7500
)

// toastUndoProps are the props for the toastUndoButton sub-component.
type toastUndoProps struct {
	// Atom is the notice atom so the Undo action can clear the toast after undo.
	Atom state.Atom[uistate.Notice]
}

// toastUndoButton is a dedicated component so that its UseEvent hook is
// registered at a stable (non-loop) call-site — required by the GoWebComponents
// framework (CLAUDE.md §"CRITICAL gotchas"). It renders the "Undo" button
// inside a Toast whose notice represents an undoable change.
func toastUndoButton(props toastUndoProps) uic.Node {
	doUndo := uic.UseEvent(func() {
		auditview.UndoFunc()
		props.Atom.Set(props.Atom.Get().Cleared())
	})

	label := uistate.T("toast.undoBtn")
	if label == "toast.undoBtn" {
		label = "Undo"
	}
	return Button(
		css.Class("toast-undo"),
		Attr("type", "button"),
		Attr("aria-label", label),
		OnClick(doUndo),
		label,
	)
}

// toastNoticeIsUndoable reports whether the notice text looks like it
// represents a delete or change that the user may want to undo. This is a
// heuristic used when the Notice model has no explicit Undoable field.
//
// Follow-up: add a Notice.Undoable bool field to uistate.Notice so callers
// (e.g. delete handlers) can set it directly, removing the need for this
// text-matching heuristic entirely.
func toastNoticeIsUndoable(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "deleted") ||
		strings.Contains(lower, "removed") ||
		strings.Contains(lower, "changed") ||
		strings.Contains(lower, "updated") ||
		strings.Contains(lower, "moved") ||
		strings.Contains(lower, "archived")
}

// Toast renders the app-wide notice surface: a single dismissible message
// pinned to the bottom of the viewport, driven by the shared uistate.Notice
// atom. Ordinary notices auto-dismiss after toastTimeoutMS; errors linger for
// toastErrTimeoutMS (and can always be dismissed by hand).
//
// Inline Undo: when the undo stack is non-empty (auditview.CanUndoFunc()) and
// the notice text matches a delete/change pattern, an inline "Undo" button
// appears. Clicking it calls auditview.UndoFunc() and dismisses the toast.
//
// Cleaner follow-up: add Notice.Undoable bool to uistate.Notice and set it
// from delete/change handlers, replacing the text-pattern heuristic below.
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
		return Div(css.Class(tw.SrOnly), Attr("role", role), Attr("aria-live", live))
	}
	cls := "toast"
	if n.Err {
		cls += " toast-err"
	}

	// Show the inline Undo button when the undo stack has something to undo and
	// the notice text indicates a destructive or mutating operation.
	// Prefer the explicit Notice.Undoable flag (set by delete/change handlers);
	// fall back to the text heuristic for callers that haven't been updated yet.
	showUndo := auditview.CanUndoFunc() && (n.Undoable || toastNoticeIsUndoable(n.Text))

	return Div(ClassStr(cls), Attr("role", role), Attr("aria-live", live),
		Span(css.Class("toast-msg"), n.Text),
		If(showUndo, uic.CreateElement(toastUndoButton, toastUndoProps{Atom: atom})),
		Button(css.Class("toast-x"), Attr("type", "button"), Attr("title", "Dismiss"), Attr("aria-label", "Dismiss"),
			OnClick(func() { atom.Set(n.Cleared()) }), "×"),
	)
}
