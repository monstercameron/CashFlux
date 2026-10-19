// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"syscall/js"
	"testing"
	"time"
)

func TestBR3DLGCoverAllCloseRestoresTriggerFocus(t *testing.T) {
	for _, reason := range []string{"Escape", "Cancel", "successful apply"} {
		t.Run(reason, func(t *testing.T) {
			focused := make(chan struct{}, 1)
			opener := js.Global().Get("Object").New()
			opener.Set("isConnected", true)
			focus := js.FuncOf(func(js.Value, []js.Value) any {
				focused <- struct{}{}
				return nil
			})
			t.Cleanup(focus.Release)
			opener.Set("focus", focus)
			coverAllOpener = opener
			t.Cleanup(func() { coverAllOpener = js.Undefined() })

			closed := false
			closeCoverAll(func() { closed = true })
			if !closed {
				t.Fatal("close callback did not run")
			}
			select {
			case <-focused:
			case <-time.After(time.Second):
				t.Fatalf("trigger did not regain focus after %s", reason)
			}
		})
	}
}
