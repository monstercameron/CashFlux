// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"syscall/js"
	"testing"
)

func TestTC4N11OverflowSubmenuFocusUsesWrapper(t *testing.T) {
	lookups := []string{}
	focused := false
	var funcs []js.Func
	makeFunc := func(fn func(js.Value, []js.Value) any) js.Func {
		f := js.FuncOf(fn)
		funcs = append(funcs, f)
		return f
	}
	t.Cleanup(func() {
		for _, f := range funcs {
			f.Release()
		}
	})
	child := js.Global().Get("Object").New()
	child.Set("focus", makeFunc(func(js.Value, []js.Value) any { focused = true; return nil }))
	menu := js.Global().Get("Object").New()
	menu.Set("querySelector", makeFunc(func(_ js.Value, args []js.Value) any {
		lookups = append(lookups, args[0].String())
		return child
	}))
	wrapper := js.Global().Get("Object").New()
	wrapper.Set("querySelector", makeFunc(func(_ js.Value, args []js.Value) any {
		lookups = append(lookups, args[0].String())
		return menu
	}))
	doc := js.Global().Get("Object").New()
	doc.Set("getElementById", makeFunc(func(_ js.Value, args []js.Value) any {
		lookups = append(lookups, args[0].String())
		if args[0].String() == "parent-wrap" {
			return wrapper
		}
		return js.Null()
	}))

	focusOverflowSubmenu(doc, "parent")
	if !focused {
		t.Fatal("opening a submenu did not focus its first item")
	}
	want := []string{"parent-wrap", ".add-submenu", `[role="menuitem"]`}
	if len(lookups) != len(want) {
		t.Fatalf("submenu lookup path = %#v, want %#v", lookups, want)
	}
	for i := range want {
		if lookups[i] != want[i] {
			t.Fatalf("submenu lookup path = %#v, want %#v", lookups, want)
		}
	}
}
