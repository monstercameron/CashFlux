//go:build js && wasm

package app

import (
	"syscall/js"

	"github.com/monstercameron/GoWebComponents/router"
)

// wireKeyboardShortcuts installs global keyboard shortcuts. Alt+1..9 jumps to the
// Nth primary navigation screen (Dashboard, Accounts, …) so the keyboard alone can
// move between sections. Registered once at boot; the listener lives for the app's
// lifetime, so its js.Func is intentionally never released.
//
// It keys off KeyboardEvent.code ("Digit1".."Digit9") so it's keyboard-layout
// independent and never matches the numpad (where Alt+number is an OS alt-code),
// and it stays out of the way while the user is typing in a field.
func wireKeyboardShortcuts() {
	doc := js.Global().Get("document")
	if doc.IsNull() || doc.IsUndefined() {
		return
	}
	nav := primaryNav() // static — the screen set doesn't change at runtime

	onKeyDown := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		e := args[0]
		if !e.Get("altKey").Bool() || e.Get("ctrlKey").Bool() || e.Get("metaKey").Bool() {
			return nil
		}
		if isEditableTarget(doc) {
			return nil
		}
		code := e.Get("code").String()
		if len(code) != 6 || code[:5] != "Digit" {
			return nil
		}
		d := code[5]
		if d < '1' || d > '9' {
			return nil
		}
		idx := int(d - '1')
		if idx >= len(nav) {
			return nil
		}
		e.Call("preventDefault")
		router.Navigate(nav[idx].Path)
		return nil
	})
	doc.Call("addEventListener", "keydown", onKeyDown)
}

// isEditableTarget reports whether focus is in a text input, so a shortcut chord
// doesn't fire (and steal the keystroke) while the user is typing.
func isEditableTarget(doc js.Value) bool {
	el := doc.Get("activeElement")
	if el.IsNull() || el.IsUndefined() {
		return false
	}
	if tag := el.Get("tagName"); !tag.IsNull() && !tag.IsUndefined() {
		switch tag.String() {
		case "INPUT", "TEXTAREA", "SELECT":
			return true
		}
	}
	if ce := el.Get("isContentEditable"); !ce.IsNull() && !ce.IsUndefined() && ce.Bool() {
		return true
	}
	return false
}
