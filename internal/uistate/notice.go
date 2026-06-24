// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/state"

// Notice is a transient, app-wide message rendered by the toast surface. Seq is
// bumped on every post so that re-posting the same Text still re-triggers the
// toast (and its auto-dismiss timer). An empty Text means "nothing showing".
type Notice struct {
	Seq  int
	Text string
	Err  bool
	// Leaving marks the notice as animating out: the toast keeps its text/styling
	// so the exit transition (.toast.hide) can play, then a short timer clears it.
	Leaving bool
}

const noticeAtomID = "app:notice"

// UseNotice returns the shared toast-notice atom. The toast component subscribes
// to it; any component can post to it via the Notice returned by With.
//
// Reading it also captures the atom into a package var so PostNotice can post a
// toast from outside a component render (e.g. a global keyboard-shortcut handler).
func UseNotice() state.Atom[Notice] {
	a := state.UseAtom(noticeAtomID, Notice{})
	capturedNotice = a
	noticeCaptured = true
	return a
}

var (
	capturedNotice state.Atom[Notice]
	noticeCaptured bool
)

// PostNotice shows a toast from outside a component render — for global callbacks
// (keyboard shortcuts, command-palette actions, undo/redo) that aren't running
// inside a component and so can't call the UseNotice hook. It is a no-op until the
// toast surface has rendered once (always true after first paint).
func PostNotice(text string, isErr bool) {
	if noticeCaptured {
		capturedNotice.Set(capturedNotice.Get().With(text, isErr))
	}
}

// With returns n advanced to show text (Seq bumped so the toast re-fires even
// for identical text). isErr styles it as an error rather than an info notice.
func (n Notice) With(text string, isErr bool) Notice {
	return Notice{Seq: n.Seq + 1, Text: text, Err: isErr}
}

// Cleared returns n with its message removed but Seq preserved, so dismissing
// or auto-expiring a toast doesn't look like a fresh post to the effect.
func (n Notice) Cleared() Notice {
	return Notice{Seq: n.Seq}
}

// MarkLeaving returns n marked as animating out: Text/Err/Seq are preserved so
// the exit transition plays and the dismiss effect isn't treated as a fresh post.
func (n Notice) MarkLeaving() Notice {
	return Notice{Seq: n.Seq, Text: n.Text, Err: n.Err, Leaving: true}
}
