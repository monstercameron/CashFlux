// SPDX-License-Identifier: MIT

//go:build js && wasm

package browserstore

import (
	"sync"
	"syscall/js"
)

// Cross-tab coherence for the in-memory cache.
//
// This package replaced localStorage with IndexedDB plus a cache loaded once at
// boot. That swap was invisible to every caller — Get/Set kept their shape — but
// it silently removed a property localStorage had and callers were relying on:
// a SHARED VIEW ACROSS TABS. localStorage reads hit one store that every tab
// sees; this cache is per-tab and frozen at that tab's boot.
//
// Code written against the old semantics therefore became quietly wrong rather
// than failing. The worst case found (2026-08-18) is in the auth layer: the
// refresh-token rotation guard re-reads the stored refresh token inside a
// cross-tab Web Lock to check whether another tab already rotated it. Against
// localStorage that read saw the other tab's write and the guard worked. Against
// a per-tab cache it can only ever see this tab's boot-time value, so the guard
// always says "unchanged", the tab replays a refresh token another tab already
// consumed, and the server — correctly — treats a replayed refresh token as a
// compromise signal and revokes the entire session family. Three open tabs could
// sign the user out of every device.
//
// Two mechanisms restore what was lost:
//
//   - Reload: re-read specific keys from IndexedDB on demand, for the moments
//     where correctness depends on seeing another tab's write RIGHT NOW.
//   - A BroadcastChannel: push every local write to the other tabs so their
//     caches converge without anyone having to ask.
//
// Both are best-effort by design. A browser without either still runs exactly as
// it does today; it just keeps the old per-tab behaviour.

const broadcastChannelName = "cashflux:store"

var (
	channel    js.Value
	watchMu    sync.Mutex
	watchers   = map[string][]func(value string, present bool){}
	onceListen sync.Once
)

// StartCrossTab opens the broadcast channel and begins applying other tabs'
// writes to this tab's cache. Safe to call more than once; a browser without
// BroadcastChannel is left with the per-tab behaviour rather than an error.
func StartCrossTab() {
	onceListen.Do(func() {
		ctor := js.Global().Get("BroadcastChannel")
		if !ctor.Truthy() {
			return
		}
		defer func() { _ = recover() }()
		channel = ctor.New(broadcastChannelName)
		channel.Set("onmessage", js.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) == 0 {
				return nil
			}
			data := args[0].Get("data")
			if !data.Truthy() {
				return nil
			}
			key := data.Get("key")
			if !key.Truthy() {
				return nil
			}
			k := key.String()
			// Re-read the authoritative value from IndexedDB rather than trust
			// the message. Two reasons, and the first is the important one:
			// nothing sensitive then travels on the channel at all (see publish),
			// and the value this tab adopts is the one that actually committed
			// rather than one tab's claim about it.
			//
			// The read runs in a GOROUTINE because Reload blocks, and this is a
			// js.Func callback: on GOOS=js/wasm the runtime is single-threaded
			// and cooperatively scheduled, so blocking here would stop the event
			// loop that has to deliver the IndexedDB callback Reload is waiting
			// for — the same deadlock internal/app/tokenlock.go documents.
			go func() {
				Reload(k)
				v, present := Get(k)
				notifyWatchers(k, v, present)
			}()
			return nil
		}))
	})
}

// publish tells the other tabs WHICH key changed — never what it changed to.
//
// The values in this store include the access and refresh tokens, and a
// BroadcastChannel is readable by anything running on the origin: an injected
// script, a same-origin iframe, a service worker. IndexedDB is readable by those
// too, so this is not a new capability for an attacker who already has script
// execution — but it is a gratuitous second copy of a credential, sitting on a
// bus that is trivially subscribed to and needs no await. Sending only the key
// keeps the secret in one place. Every recipient re-reads the value itself.
//
// Never fails a write: a browser without BroadcastChannel simply leaves the
// other tabs to find out on their next Reload or page load.
func publish(key string) {
	if !channel.Truthy() {
		return
	}
	defer func() { _ = recover() }()
	channel.Call("postMessage", map[string]any{"key": key})
}

// Watch registers fn to run when ANOTHER tab changes key. It is not called for
// this tab's own writes — the caller already knows about those, and firing on
// them would turn every local write into a re-render.
func Watch(key string, fn func(value string, present bool)) {
	if fn == nil {
		return
	}
	watchMu.Lock()
	watchers[key] = append(watchers[key], fn)
	watchMu.Unlock()
}

func notifyWatchers(key, value string, present bool) {
	watchMu.Lock()
	fns := append([]func(value string, present bool){}, watchers[key]...)
	watchMu.Unlock()
	for _, fn := range fns {
		func() {
			// One bad watcher must not stop the others, and must never take the
			// page down from inside a JS event callback.
			defer func() { _ = recover() }()
			fn(value, present)
		}()
	}
}

// Reload re-reads the named keys from IndexedDB into the cache, so this tab sees
// what other tabs have written since it booted.
//
// It BLOCKS until the read completes, which is safe only from a goroutine — not
// from inside a js.Func callback, where blocking prevents the runtime from
// pumping the event loop and the IndexedDB callback could never fire. Every
// current caller is a sync worker goroutine.
//
// Call it where correctness depends on a fresh read rather than on eventual
// convergence: before deciding whether another tab has already rotated a
// single-use credential, for instance.
func Reload(keys ...string) {
	if len(keys) == 0 || !db.Truthy() {
		return
	}
	done := make(chan struct{})
	var once sync.Once
	finish := func() { once.Do(func() { close(done) }) }
	defer func() {
		if r := recover(); r != nil {
			finish()
		}
	}()

	tx := db.Call("transaction", idbStoreName, "readonly")
	os := tx.Call("objectStore", idbStoreName)
	remaining := len(keys)
	for _, key := range keys {
		k := key
		req := os.Call("get", k)
		settle := func() {
			remaining--
			if remaining == 0 {
				finish()
			}
		}
		req.Set("onsuccess", js.FuncOf(func(js.Value, []js.Value) any {
			res := req.Get("result")
			mu.Lock()
			if res.Type() == js.TypeString {
				cache[k] = res.String()
			} else if res.IsUndefined() || res.IsNull() {
				// Absent in IndexedDB means another tab removed it — a sign-out
				// elsewhere. Dropping it here is the point: keeping a cached
				// credential that has been deleted is how a signed-out tab goes
				// on presenting a dead session as a live one.
				delete(cache, k)
			}
			mu.Unlock()
			settle()
			return nil
		}))
		req.Set("onerror", js.FuncOf(func(js.Value, []js.Value) any {
			settle()
			return nil
		}))
	}
	<-done
}

// Barrier blocks until every write issued by this tab before the call has
// committed to IndexedDB.
//
// Set persists asynchronously: it updates the cache and fires the IndexedDB put
// without waiting for the transaction to complete. That is right for ordinary
// writes and wrong at exactly one moment — when another tab is about to read
// what we just wrote in order to decide whether to reuse a single-use
// credential. Adversarial review (2026-08-18) found the window: a tab can rotate
// a refresh token, release the cross-tab lock as soon as the calls have been
// ISSUED, and let the next tab's Reload run against an IndexedDB that has not
// committed the new value yet. That tab then sees the old token, believes
// nothing has rotated, and replays it — the precise failure the lock and the
// Reload exist to prevent.
//
// It works by ordering rather than by inspection: IndexedDB runs transactions
// with overlapping scope in creation order, so a readwrite transaction created
// after the puts cannot complete before they have. Blocks, so goroutines only —
// see Reload.
func Barrier() {
	if !db.Truthy() {
		return
	}
	done := make(chan struct{})
	var once sync.Once
	finish := func() { once.Do(func() { close(done) }) }
	defer func() {
		if r := recover(); r != nil {
			finish()
		}
	}()
	tx := db.Call("transaction", idbStoreName, "readwrite")
	tx.Set("oncomplete", js.FuncOf(func(js.Value, []js.Value) any { finish(); return nil }))
	tx.Set("onerror", js.FuncOf(func(js.Value, []js.Value) any { finish(); return nil }))
	tx.Set("onabort", js.FuncOf(func(js.Value, []js.Value) any { finish(); return nil }))
	// A transaction with no request still completes; touching the store keeps the
	// scope explicit and the ordering guarantee obvious to a reader.
	tx.Call("objectStore", idbStoreName)
	<-done
}

// NotifyForTest delivers a change notification for key as if it had arrived from
// another tab, reading the current value from the cache. Exported for tests in
// dependent packages, which cannot open a second browser tab to produce the real
// thing.
func NotifyForTest(key string) {
	v, present := Get(key)
	notifyWatchers(key, v, present)
}
