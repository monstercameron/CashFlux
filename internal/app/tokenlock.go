// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"log/slog"
	"syscall/js"
)

// tokenRefreshLockName is shared by every tab in this origin. Taking one lock
// also serializes cookie refreshes, whose credential JavaScript cannot read.
const tokenRefreshLockName = "cashflux-token-refresh"

// withTokenRefreshLock starts fn only INSIDE the granted lock callback. That
// callback returns a Promise immediately so the JS event loop can deliver RPC
// and IndexedDB events while Go works. The Promise stays pending until fn has
// finished, including its durable credential write; a timeout must never let a
// second tab in while that commit is outstanding. Network calls have their own
// context deadlines, and the browser releases the lock if a tab crashes.
//
// Without Web Locks this remains best effort. A rejected lock request does not
// run fn unlocked: callers observe that their work did not run and fail closed.
// BLOCKING: call from a Go goroutine, never from a synchronous JS callback.
func withTokenRefreshLock(fn func()) {
	navigator := js.Global().Get("navigator")
	if !navigator.Truthy() || !navigator.Get("locks").Truthy() {
		fn()
		return
	}
	locks := navigator.Get("locks")
	finished := make(chan struct{})
	var executor js.Func
	executed := false
	failed := false
	callback := js.FuncOf(func(js.Value, []js.Value) any {
		executed = true
		executor = js.FuncOf(func(_ js.Value, args []js.Value) any {
			resolve := args[0]
			go func() {
				fn()
				resolve.Invoke()
			}()
			return nil
		})
		return js.Global().Get("Promise").New(executor)
	})
	settled := js.FuncOf(func(js.Value, []js.Value) any { close(finished); return nil })
	rejected := js.FuncOf(func(js.Value, []js.Value) any {
		failed = true
		close(finished)
		return nil
	})
	locks.Call("request", tokenRefreshLockName, callback).Call("then", settled, rejected)
	<-finished
	callback.Release()
	settled.Release()
	rejected.Release()
	if executed {
		executor.Release()
	}
	if failed {
		slog.Error("could not acquire the session refresh lock")
	}
}
