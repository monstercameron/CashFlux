// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"testing"

	"github.com/monstercameron/GoWebComponents/v7/interop"
)

func TestTC4N11OverflowSubmenuFocusUsesWrapper(t *testing.T) {
	lookups := []string{}
	focused := false
	child := fakeObject(t)
	fakeMethod(t, child, "focus", func(...interop.Value) any { focused = true; return nil })
	menu := fakeObject(t)
	fakeMethod(t, menu, "querySelector", func(args ...interop.Value) any {
		lookups = append(lookups, args[0].String())
		return child
	})
	wrapper := fakeObject(t)
	fakeMethod(t, wrapper, "querySelector", func(args ...interop.Value) any {
		lookups = append(lookups, args[0].String())
		return menu
	})
	doc := fakeObject(t)
	fakeMethod(t, doc, "getElementById", func(args ...interop.Value) any {
		lookups = append(lookups, args[0].String())
		if args[0].String() == "parent-wrap" {
			return wrapper
		}
		return nil
	})
	useFakeDocument(t, doc)

	focusOverflowSubmenu("parent")
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
