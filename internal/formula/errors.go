// SPDX-License-Identifier: MIT

package formula

import "fmt"

// Error is a formula error carrying the byte offset it occurred at, so an
// editor can point at the offending spot instead of regex-parsing the message.
// Pos is -1 when the error has no source position (evaluation-time errors,
// whole-input limits). Retrieve it with errors.As.
type Error struct {
	Pos int
	Msg string
}

// Error renders the message with its position, matching the package's
// long-standing "formula: … at position N" style.
func (e *Error) Error() string {
	if e.Pos >= 0 {
		return fmt.Sprintf("formula: %s at position %d", e.Msg, e.Pos)
	}
	return "formula: " + e.Msg
}

// errAt builds a positioned *Error.
func errAt(pos int, format string, args ...any) *Error {
	return &Error{Pos: pos, Msg: fmt.Sprintf(format, args...)}
}
