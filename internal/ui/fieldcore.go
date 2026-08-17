// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"sync"
	"syscall/js"
	"unicode/utf16"

	"github.com/monstercameron/GoWebComponents/v5/html"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
)

// FieldValue is what a text field's value option must be, instead of Value().
//
// # Why the value cannot be a value
//
// `value` is a special property the reconciler ALWAYS writes, and it decides
// whether to write by comparing the new prop against the PREVIOUS RENDER'S prop —
// never against what the box actually holds. State lands a render or more behind
// the keyboard, so a render resolving after the next keystroke writes its own
// older string back and the character typed in between is gone from the DOM, not
// merely from state. Measured before this existed: the assistant composer kept 13
// of 76 typed characters, scrambled, with the caret jumping to the end; three of
// four Settings fields lost text, including the OpenAI API key, where a mangled
// value reads as an auth failure rather than as a typing bug.
//
// So the value travels as an ATTRIBUTE. The reconciler diffs and sets attributes
// happily, and setting an attribute does not touch the `.value` property, so the
// clobbering is gone by construction rather than by defending against it.
// fieldSync below is what carries the attribute into the property when the
// application — rather than the user — has something to say.
//
// # Why this is a prop option and not a component
//
// The first attempt wrapped the element in a component that owned the value. It
// silently broke every call site: an On* handler created by the CALLER's UseEvent
// and passed through into a child component's element does not bind. The DOM ends
// up with an `oninput` function and real input events fire, so it looks wired —
// but the handler never reaches the application. The transactions search kept its
// text and filtered nothing; the account form typed a name and saved no account.
// A prop option has no such boundary: the element stays with its caller, the
// caller's handlers bind exactly as they always did, and migrating a field is a
// one-word change from Value to FieldValue.
func FieldValue(v string) html.PropOption {
	installFieldSync()
	return Attr(fieldValueAttr, v)
}

// fieldValueAttr carries the application's idea of a field's contents.
const fieldValueAttr = "data-cf-value"

var fieldSyncOnce sync.Once

// installFieldSync wires the two listeners that keep every field's value in step.
// It runs once, on the render thread, the first time any field renders.
func installFieldSync() {
	fieldSyncOnce.Do(func() {
		doc := js.Global().Get("document")
		if !doc.Truthy() {
			return
		}
		// A DELEGATED input listener, not one per field. Per-element listeners have
		// to be attached in a mount effect, and that effect does not reliably find
		// its element — a field whose listener never attached silently loses the very
		// protection this exists to provide. One listener on the document, in the
		// capture phase, cannot miss.
		typed := js.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) == 0 {
				return nil
			}
			if t := args[0].Get("target"); t.Truthy() {
				// Stamp that the user has edited since the last time the application
				// spoke. Cleared when a programmatic write lands.
				t.Set("__cfTyped", true)
			}
			return nil
		})
		doc.Call("addEventListener", "input", typed, true)

		// The observer fires exactly when the reconciler changes the attribute —
		// which is to say, when the application's idea of the value changed. No
		// polling, and no work at all while a screen is idle.
		obs := js.Global().Get("MutationObserver")
		if !obs.Truthy() {
			return
		}
		// One sweep per tick, never per mutated node.
		//
		// The first version walked every added subtree as it arrived, which meant a
		// wasm callback and a querySelectorAll for each of the thousands of nodes a
		// full screen render inserts. It took the ledger from usable to 28 SECONDS
		// before its first paint. Fields number in the dozens and the whole-document
		// query is trivial, so the batched sweep costs one call per render instead of
		// one per node — and it cannot miss a field, because it looks at all of them.
		var sweep js.Func
		pending := false
		sweep = js.FuncOf(func(js.Value, []js.Value) any {
			pending = false
			seedAllFields()
			return nil
		})
		cb := js.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) == 0 {
				return nil
			}
			records := args[0]
			// Attribute changes are already targeted, so they are applied directly —
			// this is the common case once a screen is up, and it touches one node.
			needSweep := false
			for i := 0; i < records.Length(); i++ {
				r := records.Index(i)
				if r.Get("type").String() == "attributes" {
					applyFieldValue(r.Get("target"))
					continue
				}
				needSweep = true
			}
			if needSweep && !pending {
				pending = true
				js.Global().Call("setTimeout", sweep, 0)
			}
			return nil
		})
		observer := obs.New(cb)
		observer.Call("observe", doc.Get("documentElement"), map[string]any{
			"subtree":         true,
			"childList":       true,
			"attributes":      true,
			"attributeFilter": []any{fieldValueAttr},
		})
		// Fields already on the page when this installs (the first render's own
		// output) never produce a mutation record, so they are seeded directly.
		seedAllFields()
	})
}

// seedAllFields fills every field currently on the page. One document-wide query
// is cheaper than walking each inserted subtree, and it cannot miss a field.
func seedAllFields() {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return
	}
	found := doc.Call("querySelectorAll", "["+fieldValueAttr+"]")
	for i := 0; i < found.Length(); i++ {
		applyFieldValue(found.Index(i))
	}
}

// applyFieldValue carries the attribute into the element's value, unless the user
// is the one who should be deciding what it says.
func applyFieldValue(el js.Value) {
	if !el.Truthy() || el.Get("getAttribute").IsUndefined() {
		return
	}
	want := el.Call("getAttribute", fieldValueAttr)
	if !want.Truthy() && want.Type() != js.TypeString {
		return
	}
	v := want.String()
	if el.Get("value").String() == v {
		el.Set("__cfTyped", false)
		return
	}
	// THE RULE: while the box has focus and the user has typed since the last time
	// the application wrote to it, the box wins.
	//
	// There is no way to tell a stale value from a new instruction by looking at
	// the value — a render arriving mid-word carries a value that is simply OLD,
	// and old values and new instructions are routinely the same string. Two
	// earlier attempts tried and failed: comparing against the last keystroke
	// deletes the character typed since (the original bug rebuilt inside its own
	// fix), and comparing against every keystroke seen refuses a legitimate clear
	// once the user has backspaced to empty. Authority answers it without history.
	//
	// The "__cfTyped since we last wrote" half matters as much as the focus half: a
	// form that clears itself on submit does so while the field it was typed into
	// still holds focus, and refusing that write would leave the submitted text
	// sitting in the box.
	if isFocused(el) && el.Get("__cfTyped").Truthy() {
		return
	}
	writeFieldValue(el, v)
	el.Set("__cfTyped", false)
}

// isFocused reports whether el is the element the user is currently typing into.
func isFocused(el js.Value) bool {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return false
	}
	return doc.Get("activeElement").Equal(el)
}

// selectableTypes are the input types whose selection API may be read. The list is
// a whitelist rather than a blacklist because getting it wrong is not a cosmetic
// bug: reading selectionStart on a date, number, email or colour input THROWS in
// Chrome, and an exception crossing back into wasm takes the page down with it. A
// field whose caret cannot be read simply keeps the browser's own behaviour.
var selectableTypes = map[string]bool{
	"": true, "text": true, "search": true, "url": true, "tel": true, "password": true,
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
// upper-casing, stripping punctuation — writes back as the user types by design,
// and a caret that jumps to the end makes such a field unusable for editing
// anything but the last character.
func writeFieldValue(el js.Value, v string) {
	doc := js.Global().Get("document")
	focused := doc.Truthy() && doc.Get("activeElement").Equal(el)
	caret := -1
	if focused && supportsSelection(el) {
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
	// Measured in Go, not through JS. `value` comes back as a JS string, and a
	// string is not an object in syscall/js: both Get("length") and Length() panic
	// on one and take the whole Go runtime down with them, so every field seeded
	// while focused was a live crash. We already hold the string we just wrote, so
	// there is nothing to ask the DOM for.
	if n := utf16Len(v); caret > n {
		caret = n
	}
	el.Call("setSelectionRange", caret, caret)
}

// utf16Len is a string's length in UTF-16 code units — what the selection API
// counts in. Go's len() would be wrong the moment somebody types a currency
// symbol, and a rune count would be wrong for anything outside the BMP.
func utf16Len(s string) int {
	return len(utf16.Encode([]rune(s)))
}
