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
}

const noticeAtomID = "app:notice"

// UseNotice returns the shared toast-notice atom. The toast component subscribes
// to it; any component can post to it via the Notice returned by With.
func UseNotice() state.Atom[Notice] {
	return state.UseAtom(noticeAtomID, Notice{})
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
