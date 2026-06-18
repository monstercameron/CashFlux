//go:build js && wasm

package screens

import (
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// errAttrs returns the ARIA attributes that tie a form input to its validation
// error: when msg is non-empty, the input is marked invalid (aria-invalid) and
// points at the error element — which must carry id=errID — via aria-describedby,
// so a screen reader announces the error when focus lands on the field, not only
// once via the error's role="alert" (WCAG 3.3.1/4.1.2). Spread the result into an
// input's args; it's nil (a no-op) when there's no error.
//
// It returns no hook-registering options, so appending it after an input's
// On*-handler keeps the hook count stable whether or not an error is present.
func errAttrs(errID, msg string) []any {
	if msg == "" {
		return nil
	}
	return []any{Attr("aria-describedby", errID), Attr("aria-invalid", "true")}
}

// errText renders a form-level validation error with a stable id (so inputs can
// reference it via errAttrs) and role="alert" (so it's announced when it appears),
// rendering nothing when msg is blank — the drop-in replacement for the previous
// inline If(msg != "", P(Class("err"), …)) pattern.
func errText(errID, msg string) ui.Node {
	return If(msg != "", P(Class("err"), Attr("role", "alert"), Attr("id", errID), msg))
}
