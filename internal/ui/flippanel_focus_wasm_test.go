// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"testing"

	"syscall/js"
)

func TestFocusRestoreUsesReplacementForRemovedOpener(t *testing.T) {
	object := js.Global().Get("Object")
	previous, replacement, doc := object.New(), object.New(), object.New()
	previous.Set("isConnected", false)
	previous.Set("getAttribute", testJSFunc(t, func(_ js.Value, args []js.Value) any {
		if args[0].String() == "data-testid" {
			return "budgets-adjust-all"
		}
		return js.Null()
	}))
	replacement.Set("getAttribute", testJSFunc(t, func(_ js.Value, args []js.Value) any {
		if args[0].String() == "data-testid" {
			return "budgets-adjust-all"
		}
		return js.Null()
	}))
	doc.Set("getElementById", testJSFunc(t, func(js.Value, []js.Value) any { return js.Null() }))
	doc.Set("querySelectorAll", testJSFunc(t, func(js.Value, []js.Value) any { return []any{replacement} }))

	if got := focusRestoreTarget(doc, previous); !got.Equal(replacement) {
		t.Fatal("focus restoration did not resolve the current control that replaced the opener")
	}
}

func TestFocusRestoreKeepsConnectedOpener(t *testing.T) {
	object := js.Global().Get("Object")
	previous, doc := object.New(), object.New()
	previous.Set("isConnected", true)
	if got := focusRestoreTarget(doc, previous); !got.Equal(previous) {
		t.Fatal("focus restoration replaced a still-connected opener")
	}
}

func TestFlipPanelInitialFocusPrefersAutofocusThenFirstField(t *testing.T) {
	object := js.Global().Get("Object")
	button, field, marked := object.New(), object.New(), object.New()
	button.Set("tagName", "BUTTON")
	field.Set("tagName", "INPUT")
	marked.Set("tagName", "BUTTON")
	button.Set("hasAttribute", testJSFunc(t, func(js.Value, []js.Value) any { return false }))
	field.Set("hasAttribute", testJSFunc(t, func(js.Value, []js.Value) any { return false }))
	marked.Set("hasAttribute", testJSFunc(t, func(_ js.Value, args []js.Value) any { return args[0].String() == "autofocus" }))

	if got := flipPanelInitialFocusTarget([]js.Value{button, field}); !got.Equal(field) {
		t.Fatal("initial focus did not prefer the first field to the header button")
	}
	if got := flipPanelInitialFocusTarget([]js.Value{button, field, marked}); !got.Equal(marked) {
		t.Fatal("initial focus did not honor an explicit autofocus field")
	}
}

func testJSFunc(t *testing.T, fn func(js.Value, []js.Value) any) js.Func {
	t.Helper()
	owned := js.FuncOf(fn)
	t.Cleanup(owned.Release)
	return owned
}
