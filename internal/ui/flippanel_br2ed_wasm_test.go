// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"testing"

	"syscall/js"
)

func TestFlipPanelResolvesBudgetDialogOpenerAfterReplacement(t *testing.T) {
	for _, attr := range []string{"id", "data-testid"} {
		t.Run(attr, func(t *testing.T) {
			object := js.Global().Get("Object")
			previous, replacement, doc := object.New(), object.New(), object.New()
			previous.Set("isConnected", false)
			previous.Set("getAttribute", testJSFunc(t, func(_ js.Value, args []js.Value) any {
				if args[0].String() == attr {
					if attr == "id" {
						return "budget-kebab-groceries"
					}
					return "budget-kebab-groceries"
				}
				return js.Null()
			}))
			replacement.Set("getAttribute", testJSFunc(t, func(_ js.Value, args []js.Value) any {
				if args[0].String() == attr {
					return "budget-kebab-groceries"
				}
				return js.Null()
			}))
			doc.Set("getElementById", testJSFunc(t, func(_ js.Value, args []js.Value) any {
				if args[0].String() == "budget-kebab-groceries" {
					return replacement
				}
				return js.Null()
			}))
			doc.Set("querySelectorAll", testJSFunc(t, func(js.Value, []js.Value) any {
				return []any{replacement}
			}))
			if got := focusRestoreTarget(doc, previous); !got.Equal(replacement) {
				t.Fatalf("replacement opener lookup by %s failed", attr)
			}
		})
	}
}
