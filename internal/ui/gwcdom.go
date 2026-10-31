// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import "github.com/monstercameron/GoWebComponents/v7/interop"

// domElement finds an element by id through GWC's typed document interface. The typed Element
// covers focus, listeners, geometry and scroll metrics; anything it does not expose goes
// through domNode.
func domElement(id string) (interop.Element, bool) {
	doc, err := interop.CurrentDocument()
	if err != nil {
		return interop.Element{}, false
	}
	element, found, err := doc.ElementByID(id)
	if err != nil || !found {
		return interop.Element{}, false
	}
	return element, true
}

// domNode finds an element by id and returns it as a raw interop.Value. The typed Element has no
// contains, querySelectorAll, classList or style accessors, so traversal and style writes go
// through domValue and domSet instead.
func domNode(id string) interop.Value {
	return domValue(domDocument(), "getElementById", id)
}

// domRoot yields the document the helpers work against. Tests replace it with a fake document
// (see useFakeDocument), so DOM lookups can be checked without a page.
var domRoot = func() interop.Value {
	global, err := interop.GlobalThis()
	if err != nil {
		return interop.Value{}
	}
	return global.Get("document")
}

// domDocument returns the page's document as an interop.Value. Off the page it is empty, so
// Truthy reports false.
func domDocument() interop.Value {
	return domRoot()
}

// domValue calls a method on a DOM value and returns the result. A failed call yields the zero
// Value, which reports Truthy() false, so callers can test the result without handling the error.
func domValue(target interop.Value, method string, args ...any) interop.Value {
	result, err := target.Call(method, args...)
	if err != nil {
		return interop.Value{}
	}
	return result
}

// domSet writes a property on a DOM value. The error is ignored, which suits style and attribute
// writes on a node that may have already left the page.
func domSet(target interop.Value, property string, value any) {
	_ = target.Set(property, value)
}
