// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"syscall/js"

	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

// fieldCoreProps configures the element fieldCore owns.
type fieldCoreProps struct {
	// Multiline renders a <textarea> instead of an <input>.
	Multiline bool
	// Numeric marks a type="number" field, whose selection API cannot be read.
	Numeric bool
	// Value is what the APPLICATION believes the field holds.
	Value string
	// OnInput receives every keystroke. It may be nil for a read-only field.
	OnInput func(string)
	// Args are the element's options — classes, placeholder, id, testids, aria.
	Args []any
}

// fieldCore is the one place in the app that owns a text field's value.
//
// Binding a text box's `value` to state that changes per keystroke is the single
// most damaging pattern available in this framework. `value` is a special property
// the reconciler ALWAYS writes, and it decides whether to write by comparing the
// new prop against the PREVIOUS RENDER'S prop — never against what the box
// actually holds. So a render that resolves after the next keystroke writes its
// own older string back, and the character typed in between is gone from the DOM,
// not merely from state. Measured on the assistant composer before this existed:
// 76 characters typed, 13 landed, scrambled; the caret jumping to the end when
// editing mid-draft; three of four Settings fields losing text, including the
// OpenAI API key, where a silently mangled value reads as an auth failure rather
// than as a typing bug.
//
// The fix is to take `value` away from the reconciler entirely. This component
// renders NO Value option, so the diff has nothing to write, and pushes the value
// in from a layout effect instead — synchronously after the DOM mutation and
// before paint, so the initial value is never seen to arrive late.
//
// The rule that makes it correct is about AUTHORITY, not about values: while the
// box has focus and its contents changed since the last render, the box wins. See
// the layout effect for why no comparison of values can work here.
func fieldCore(p fieldCoreProps) uic.Node {
	// A private handle on the element. It is a data attribute rather than an id
	// because call sites pass ids of their own (#cf-chat-input is addressed by
	// name from the keyboard-history handler and by the e2e suite), and a second
	// id would either collide with theirs or silently replace it.
	handle := uic.UseId()
	// seeded is the element this instance has already given a starting value to.
	// Comparing the element rather than holding a bool covers the case where the
	// reconciler replaces the node: the replacement is blank and must be filled.
	seeded := uic.UseRef(js.Null())
	// lastDom is what the box held at the previous render, which is how this tells
	// "the user is typing right now" from "the user has stopped".
	lastDom := uic.UseRef("")

	onInput := uic.UseEvent(func(v string) {
		if p.OnInput != nil {
			p.OnInput(v)
		}
	})

	// Runs after every render, because a re-render is exactly when a programmatic
	// change arrives and there is no dependency that reliably describes one.
	uic.UseLayoutEffect(func() func() {
		el := fieldElement(handle)
		if !el.Truthy() {
			return nil
		}
		cur := el.Get("value").String()
		prev := lastDom.Get()
		lastDom.Set(cur)
		// First sight of this element: give it its starting value, focused or not.
		// An autofocus field is focused before it is ever filled, so deferring to the
		// focus rule below would leave it permanently blank.
		if !seeded.Get().Equal(el) {
			seeded.Set(el)
			writeFieldValue(el, p.Value, p.Numeric)
			lastDom.Set(p.Value)
			return nil
		}
		if cur == p.Value {
			return nil // already agrees; nothing to do
		}
		// THE RULE: while the box has focus, the box wins.
		//
		// Everything else here is bookkeeping; this is the fix. State lands a render
		// or more behind the keyboard, so a render arriving mid-word carries a value
		// that is simply OLD — and there is no way to tell an old value from a new
		// instruction by looking at the value, because they are often the same string.
		// Two earlier attempts tried: comparing against the last keystroke (deletes
		// the character typed since — the original bug rebuilt inside its own fix,
		// 55 of 59), then against every keystroke seen (needs a listener that is not
		// reliably attached, and refuses a legitimate clear once the user has
		// backspaced to empty). Focus answers it without history: if the user is in
		// the field, they are the authority on what it contains. Programmatic writes
		// land when they are not — on mount, on load, on reopen — which is when
		// virtually all of them happen anyway. A screen that must overwrite a focused
		// field writes the element directly, as the assistant composer does on send.
		// "Focused" alone would be too blunt: a form that clears itself on submit does
		// so while the field it was typed into still holds focus, and refusing that
		// write leaves the submitted text sitting in the box. The extra condition is
		// whether the box CHANGED since the last render — that is what separates a
		// user mid-word from a user who has stopped and had something happen to them.
		if isFocused(el) && cur != prev {
			return nil
		}
		writeFieldValue(el, p.Value, p.Numeric)
		lastDom.Set(p.Value)
		return nil
	})

	args := append([]any{Attr("data-cf-field", handle)}, p.Args...)
	// Only the typed-props path gets a handler from here. In the Field path the call
	// site's own OnInput is already in Args, and appending a second one would win the
	// prop diff and silently replace it — the field would keep its text and stop
	// telling the screen about it, which is a worse bug than the one being fixed.
	// Its keystrokes are recorded by the DOM listener above instead.
	if p.OnInput != nil {
		args = append(args, OnInput(onInput))
	}
	if p.Multiline {
		return Textarea(args...)
	}
	return Input(args...)
}

// fieldElement finds the element a fieldCore instance owns. It is addressed by a
// private data attribute rather than an id because call sites bring ids of their
// own — #cf-chat-input is reached by name from the keyboard-history handler and
// from the e2e suite — and a second id would collide with or replace theirs.
func fieldElement(handle string) js.Value {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return js.Null()
	}
	return doc.Call("querySelector", "[data-cf-field='"+handle+"']")
}

// isFocused reports whether el is the element the user is currently typing into.
func isFocused(el js.Value) bool {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return false
	}
	return doc.Get("activeElement").Equal(el)
}

// Field is the migration form of a text input: it takes the value that used to be
// passed as a Value() option, plus every other option the call site already had —
// classes, ids, aria, testids, and its existing OnInput handler, whatever type
// that handler happens to be.
//
//	Input(css.Class("x"), Type("text"), Value(s.Get()), OnInput(onS))
//	uiw.Field(s.Get(), css.Class("x"), Type("text"), OnInput(onS))
//
// Prefer TextInput for new code, which carries the shared .field styling. Field
// exists so that moving an existing hand-rolled input onto the component that
// keeps its text is a one-line change and nothing about the surrounding screen
// has to be retyped.
func Field(value string, opts ...any) uic.Node {
	return uic.CreateElement(fieldCore, fieldCoreProps{Value: value, Args: opts})
}

// AreaField is Field for a <textarea>.
func AreaField(value string, opts ...any) uic.Node {
	return uic.CreateElement(fieldCore, fieldCoreProps{Value: value, Multiline: true, Args: opts})
}

// NumField is Field for a number input, whose selection API cannot be read.
func NumField(value string, opts ...any) uic.Node {
	return uic.CreateElement(fieldCore, fieldCoreProps{Value: value, Numeric: true, Args: opts})
}

// selectableTypes are the input types whose selection API may be read. The list
// is a whitelist rather than a blacklist because getting it wrong is not a
// cosmetic bug: reading selectionStart on a date, number, email or colour input
// THROWS in Chrome, and an exception crossing back into wasm takes the page down
// with it. A field whose caret cannot be read simply keeps the browser's own
// behaviour, which is the correct fallback.
var selectableTypes = map[string]bool{
	"": true, "text": true, "search": true, "url": true, "tel": true, "password": true,
	"textarea": true,
}

// supportsSelection reports whether el's caret position can safely be read.
func supportsSelection(el js.Value) bool {
	if el.Get("tagName").String() == "TEXTAREA" {
		return true
	}
	return selectableTypes[el.Get("type").String()]
}

// writeFieldValue puts v in the element without throwing the caret to the end.
//
// The caret matters because a field whose value the app transforms — trimming,
// upper-casing, stripping punctuation — writes back on every keystroke by design,
// and a caret that jumps to the end makes such a field unusable for editing
// anything but the last character. numeric fields are exempt: reading
// selectionStart on an input[type=number] throws in Chrome.
func writeFieldValue(el js.Value, v string, numeric bool) {
	if el.Get("value").String() == v {
		return
	}
	doc := js.Global().Get("document")
	focused := doc.Truthy() && doc.Get("activeElement").Equal(el)
	caret := -1
	if focused && !numeric && supportsSelection(el) {
		if s := el.Get("selectionStart"); s.Type() == js.TypeNumber {
			caret = s.Int()
		}
	}
	el.Set("value", v)
	if caret < 0 {
		return
	}
	// Clamp in UTF-16 units, which is what the selection API counts in — Go's len
	// would be wrong the moment somebody types a currency symbol or an emoji.
	if n := el.Get("value").Get("length").Int(); caret > n {
		caret = n
	}
	el.Call("setSelectionRange", caret, caret)
}
