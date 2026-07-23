// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"syscall/js"
	"time"
)

// tokenRefreshLockName is the Web Locks API lock name every tab contends for
// before performing a token refresh (TODOS.md C424). Using a single named
// lock across every open CashFlux tab against the same origin is exactly the
// point: it turns "N tabs each notice an about-to-expire token and race the
// server for a refresh" into "one tab refreshes, the rest wait and reuse its
// result."
const tokenRefreshLockName = "cashflux-token-refresh"

// tokenRefreshLockTimeout bounds how long a tab waits for its OWN refresh
// attempt to finish while holding the lock, not how long other tabs wait for
// the lock itself. It exists so a tab that crashes, is killed, or hangs
// mid-refresh cannot strand the lock and starve every other tab from ever
// refreshing again: once this elapses the holder's callback returns (and the
// browser releases the lock to the next waiter) even if the refresh attempt
// is still running in the background. The browser's own Web Locks API
// additionally releases a lock outright the instant the holding tab/worker is
// closed or crashes — this timeout only covers the "still open but wedged"
// case that guarantee doesn't reach.
//
// This MUST stay comfortably above doRefreshAccessToken's own dial/RPC
// timeout (15s at last check). If it were shorter, a merely-slow (not
// wedged) refresh could still be genuinely in flight — holding the
// not-yet-consumed refresh token — when this timeout releases the lock
// early; a second tab would then acquire it, see the same still-current
// refresh token, and fire its own concurrent RefreshToken call. Whichever
// one loses that race trips the server's single-use reuse detection and
// revokes the WHOLE session family — exactly the false-positive this lock
// exists to prevent. Keeping this above the RPC timeout means "the RPC
// itself gave up" always wins the race against "the lock's safety valve
// fired," so the lock only ever lets a second tab in for a truly wedged
// (not just slow) holder.
const tokenRefreshLockTimeout = 20 * time.Second

// withTokenRefreshLock runs fn while holding the cross-tab token-refresh
// lock, so a proactive or reactive refresh triggered independently in
// several open tabs never races the server: only the tab that wins the lock
// actually calls RefreshToken, and — because shouldRefreshNow re-checks the
// stored token's freshness right before doing any work — a tab that only
// *waited* for the lock and found the token was already refreshed by the
// time it got in simply does nothing.
//
// Browsers without the Web Locks API (older WebViews) fall back to running
// fn directly with no cross-tab coordination — best effort, not a hard
// requirement, since a lost race there just means an extra refresh call, not
// data loss.
func withTokenRefreshLock(fn func()) {
	locks := js.Global().Get("navigator").Get("locks")
	if !locks.Truthy() {
		fn()
		return
	}

	fnDone := make(chan struct{})
	go func() {
		defer close(fnDone)
		fn()
	}()

	// The lock callback MUST return a Promise and let Go resolve it when the
	// work is done — it must never block a goroutine inside its own
	// synchronous js.Func invocation. On GOOS=js/wasm the runtime is
	// single-threaded and cooperatively scheduled: while a js.Func callback is
	// parked on a Go channel, the runtime cannot pump the JS event loop, so the
	// WebSocket "open" event that doRefreshAccessToken's concurrently-dialing
	// gRPC transport is waiting on is never delivered. The dial then never
	// reaches READY and the refresh RPC fails its whole deadline with
	// "DeadlineExceeded ... waiting for connections to become ready" — even
	// though a raw WebSocket to the same origin opens in single-digit ms
	// (reproduced live, 3/3, before this fix). Returning a Promise instead
	// keeps the callback non-blocking: the browser holds the named lock for
	// exactly as long as that Promise stays pending, so the cross-tab mutual
	// exclusion this guard exists for is fully preserved, while the Go
	// scheduler stays free to service the dial.
	released := make(chan struct{})
	var callback, executor js.Func
	callback = js.FuncOf(func(this js.Value, args []js.Value) any {
		executor = js.FuncOf(func(_ js.Value, promiseArgs []js.Value) any {
			resolve := promiseArgs[0]
			go func() {
				select {
				case <-fnDone:
				case <-time.After(tokenRefreshLockTimeout):
					// fn is still running, but do not hold the lock (and every
					// other tab's refresh) hostage to it — release now by
					// resolving the Promise. fn keeps running to completion in
					// its own goroutine regardless.
				}
				resolve.Invoke()
				close(released)
			}()
			return nil
		})
		return js.Global().Get("Promise").New(executor)
	})
	locks.Call("request", tokenRefreshLockName, callback)
	<-released
	callback.Release()
	executor.Release()
}
