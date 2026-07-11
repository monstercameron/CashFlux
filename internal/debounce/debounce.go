// SPDX-License-Identifier: MIT

// Package debounce is the project's standardized trailing-edge debouncer: it defers
// work until a burst of events (keystrokes, drags, resize ticks) has settled, so the
// expensive tail — a store write, a full re-render/recompute, a persist — runs once
// instead of on every event.
//
// It is keyed: each Call for a given key cancels the previous pending call for that key,
// so a per-field key (e.g. an entity id) keeps fields independent without any per-call
// state to thread through renders. That makes it a clean fit for as-you-type inputs that
// commit to the store:
//
//	// on every keystroke — coalesces into one commit ~300ms after typing settles:
//	debounce.Call("acct-savings:"+id, 300*time.Millisecond, func() { commit(v) })
//	// on blur/change — flush the pending debounce and commit immediately:
//	debounce.Flush("acct-savings:"+id); commit(v)
//
// Built on time.AfterFunc, so it works in the wasm/JS single-threaded runtime (the timer
// fires on the event loop) and is unit-testable on native Go. The mutex guards the shared
// timer map; callers may invoke from any goroutine.
package debounce

import (
	"sync"
	"time"
)

var (
	mu     sync.Mutex
	timers = map[string]*time.Timer{}
)

// Call schedules fn to run after delay of quiet for key, cancelling any call already
// pending for the same key (trailing edge). A delay <= 0 runs fn synchronously now and
// clears any pending call for the key.
func Call(key string, delay time.Duration, fn func()) {
	mu.Lock()
	if t := timers[key]; t != nil {
		t.Stop()
		delete(timers, key)
	}
	if delay <= 0 {
		mu.Unlock()
		fn()
		return
	}
	timers[key] = time.AfterFunc(delay, func() {
		mu.Lock()
		delete(timers, key)
		mu.Unlock()
		fn()
	})
	mu.Unlock()
}

// Flush cancels any pending Call for key WITHOUT running it. Call it right before doing
// the work immediately (e.g. committing on blur) so the debounced call doesn't also
// fire, or on teardown to drop a pending call. It is a no-op when nothing is pending and
// reports whether it cancelled something.
func Flush(key string) bool {
	mu.Lock()
	defer mu.Unlock()
	if t := timers[key]; t != nil {
		t.Stop()
		delete(timers, key)
		return true
	}
	return false
}

// Pending reports whether a Call is currently scheduled for key (mainly for tests).
func Pending(key string) bool {
	mu.Lock()
	defer mu.Unlock()
	return timers[key] != nil
}
