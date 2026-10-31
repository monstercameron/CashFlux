// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"testing"

	"github.com/monstercameron/GoWebComponents/v7/interop"
)

// fakeObject returns a bare JavaScript object built through GWC's interop wrapper, so a test
// can stand in for a DOM node and record what the helpers call on it.
func fakeObject(t *testing.T) interop.Value {
	t.Helper()
	global, err := interop.GlobalThis()
	if err != nil {
		t.Fatal(err)
	}
	obj, err := global.Get("Object").Call("create", nil)
	if err != nil {
		t.Fatal(err)
	}
	return obj
}

// fakeMethod installs a JavaScript function on a fake object.
func fakeMethod(t *testing.T, obj interop.Value, name string, fn func(args ...interop.Value) any) {
	t.Helper()
	if _, err := obj.SetFunction(name, fn); err != nil {
		t.Fatal(err)
	}
}

// useFakeDocument swaps the document the helpers read for doc, and restores the page's own
// document when the test ends.
func useFakeDocument(t *testing.T, doc interop.Value) {
	t.Helper()
	previous := domRoot
	domRoot = func() interop.Value { return doc }
	t.Cleanup(func() { domRoot = previous })
}
