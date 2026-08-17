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
// The rule that makes it correct is the early return below: never write when the
// application's value is merely an ECHO of what the user last typed. That is the
// exact case the reconciler got wrong. If the user has typed a character since the
// echo was recorded, the box holds echo+char while the app still holds echo — and
// writing there is precisely how the character was being deleted.
func fieldCore(p fieldCoreProps) uic.Node {
	// A private handle on the element. It is a data attribute rather than an id
	// because call sites pass ids of their own (#cf-chat-input is addressed by
	// name from the keyboard-history handler and by the e2e suite), and a second
	// id would either collide with theirs or silently replace it.
	handle := uic.UseId()
	// typed records EVERY value the user has produced since the last programmatic
	// write, not merely the most recent one — and that distinction is the whole
	// component.
	//
	// State updates land a render or more behind the keyboard, so by the time the
	// render carrying "Th" arrives the user may already have typed "The". Comparing
	// the incoming value against the latest keystroke alone therefore says "the app
	// disagrees with the box, write it" and deletes the "e" — which is the original
	// bug rebuilt inside its own fix. Measured: still 55 of 59 characters. A value
	// the user typed at ANY point since the last programmatic write is an echo, however
	// stale, and must never be written back.
	typed := uic.UseRef(map[string]bool{})

	onInput := uic.UseEvent(func(v string) {
		log := typed.Get()
		// Bounded so a long editing session cannot grow it without limit; the window
		// only has to outlive the render lag, which is a frame or two.
		if len(log) > 256 {
			log = map[string]bool{}
			typed.Set(log)
		}
		log[v] = true
		if p.OnInput != nil {
			p.OnInput(v)
		}
	})

	// Runs after every render, because a re-render is exactly when a programmatic
	// change arrives and there is no dependency that reliably describes one.
	uic.UseLayoutEffect(func() func() {
		el := js.Global().Get("document").Call("querySelector", "[data-cf-field='"+handle+"']")
		if !el.Truthy() {
			return nil
		}
		if el.Get("value").String() == p.Value {
			return nil // already agrees; nothing to do
		}
		if typed.Get()[p.Value] {
			return nil // a value the user typed, arriving late — never write it back
		}
		// A programmatic write supersedes everything typed before it, so the echo log
		// starts again: otherwise clearing to "" would be refused forever once the
		// user had backspaced to empty even once.
		typed.Set(map[string]bool{})
		writeFieldValue(el, p.Value, p.Numeric)
		return nil
	})

	args := append([]any{Attr("data-cf-field", handle)}, p.Args...)
	args = append(args, OnInput(onInput))
	if p.Multiline {
		return Textarea(args...)
	}
	return Input(args...)
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
	if focused && !numeric {
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
