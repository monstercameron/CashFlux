// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/uistate"
)

// relabelBootChrome translates the two install affordances that live in
// web/index.html rather than in the Go UI tree (C361).
//
// The boot skeleton's own copy ("Getting your money in order…") is replaced by
// the app the moment wasm mounts, so it is not worth translating — nobody reads
// it for longer than a paint. The install button and the iOS "Add to Home
// Screen" hint are different: they are shown by page script AFTER boot, they
// persist for as long as the browser offers them, and they were the last
// user-facing English in the app that the language setting could not reach.
//
// They are relabelled from Go rather than by adding a language map to the page
// script, because index.html's JavaScript is deliberately frozen — every fix
// goes in Go. Missing nodes are skipped silently: the install button only exists
// on browsers that fire beforeinstallprompt, and the hint only on iOS Safari.
func relabelBootChrome() {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return
	}
	setText := func(id, text string) {
		if n := doc.Call("getElementById", id); n.Truthy() {
			n.Set("textContent", text)
		}
	}
	setAttr := func(id, attr, val string) {
		if n := doc.Call("getElementById", id); n.Truthy() {
			n.Call("setAttribute", attr, val)
		}
	}
	setText("installBtn", uistate.T("boot.install"))
	setAttr("installBtn", "aria-label", uistate.T("boot.install"))
	setAttr("iosHintClose", "aria-label", uistate.T("boot.iosHintDismiss"))

	// The iOS hint's body is prose around a close button, so only the text nodes
	// are replaced — rewriting the element's whole content would delete the
	// button and its listener with it.
	if n := doc.Call("getElementById", "iosHintBanner"); n.Truthy() {
		kids := n.Get("childNodes")
		body := uistate.T("boot.iosHint")
		replaced := false
		for i := kids.Length() - 1; i >= 0; i-- {
			k := kids.Index(i)
			// nodeType 3 is a text node; 1 is the <strong>s the hint wraps.
			switch k.Get("nodeType").Int() {
			case 3:
				if !replaced {
					k.Set("textContent", body)
					replaced = true
				} else {
					k.Set("textContent", "")
				}
			case 1:
				if k.Get("id").String() != "iosHintClose" {
					k.Set("textContent", "")
				}
			}
		}
	}
}
