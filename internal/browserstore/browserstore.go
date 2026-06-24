// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package browserstore is the app's single persistence primitive: an IndexedDB-
// backed key-value store that replaces localStorage entirely, so CashFlux depends
// on no localStorage at all. It keeps an in-memory cache (loaded from IndexedDB
// once at boot, migrating any legacy localStorage entries in) so callers get a
// synchronous Get/Set/Remove API — the same shape localStorage had — while writes
// persist to IndexedDB asynchronously.
//
// Being a low-level package (no app/uistate imports), it is importable everywhere —
// the SQLite-dataset persistence, the workspace registry, bootstrap flags, i18n,
// and API keys all route through it, with no import cycles.
package browserstore

import (
	"strings"
	"sync"
	"syscall/js"
)

const (
	// idbName is intentionally distinct from the artifact store's "cashflux" database
	// (internal/artifactstore) — sharing a name + version means whichever opens first
	// creates its object store and the other's onupgradeneeded never fires, so its
	// store is missing and transactions throw. A separate database avoids that.
	idbName      = "cashflux-kv"
	idbStoreName = "kv"
	keyPrefix    = "cashflux:"
)

var (
	mu    sync.Mutex
	cache = map[string]string{}
	db    js.Value
	ready bool
)

// Init opens IndexedDB, loads all entries into the in-memory cache, and migrates
// any legacy localStorage "cashflux:" keys in on first run. It BLOCKS until the
// load completes — safe because callers invoke it from the boot goroutine after the
// wasm runtime is initialized (never at package-init). Falls back to a localStorage
// mirror if IndexedDB is unavailable, so the app still runs. Idempotent.
func Init() {
	if ready {
		return
	}
	defer func() { ready = true }()
	idb := js.Global().Get("indexedDB")
	if !idb.Truthy() {
		mirrorLocalStorage()
		return
	}
	done := make(chan struct{})
	var once sync.Once
	finish := func() { once.Do(func() { close(done) }) }

	req := idb.Call("open", idbName, 1)
	req.Set("onupgradeneeded", js.FuncOf(func(js.Value, []js.Value) any {
		d := req.Get("result")
		if !d.Get("objectStoreNames").Call("contains", idbStoreName).Bool() {
			d.Call("createObjectStore", idbStoreName)
		}
		return nil
	}))
	req.Set("onerror", js.FuncOf(func(js.Value, []js.Value) any {
		mirrorLocalStorage()
		finish()
		return nil
	}))
	req.Set("onsuccess", js.FuncOf(func(js.Value, []js.Value) any {
		// Any failure here (e.g. a missing object store from an older DB) must fall
		// back to the localStorage mirror, never crash the app.
		defer func() {
			if r := recover(); r != nil {
				db = js.Value{}
				mirrorLocalStorage()
				finish()
			}
		}()
		db = req.Get("result")
		// Guard: if the expected object store is absent, don't transact (would throw).
		if !db.Get("objectStoreNames").Call("contains", idbStoreName).Bool() {
			db = js.Value{}
			mirrorLocalStorage()
			finish()
			return nil
		}
		tx := db.Call("transaction", idbStoreName, "readonly")
		os := tx.Call("objectStore", idbStoreName)
		keysReq := os.Call("getAllKeys")
		valsReq := os.Call("getAll")
		valsReq.Set("onsuccess", js.FuncOf(func(js.Value, []js.Value) any {
			keys, vals := keysReq.Get("result"), valsReq.Get("result")
			n := keys.Get("length").Int()
			mu.Lock()
			for i := 0; i < n; i++ {
				cache[keys.Index(i).String()] = vals.Index(i).String()
			}
			empty := len(cache) == 0
			mu.Unlock()
			if empty {
				migrateLocalStorage()
			}
			finish()
			return nil
		}))
		valsReq.Set("onerror", js.FuncOf(func(js.Value, []js.Value) any {
			mirrorLocalStorage()
			finish()
			return nil
		}))
		return nil
	}))
	<-done
}

// mirrorLocalStorage copies localStorage cashflux:* entries into the cache without
// touching localStorage — the no-IndexedDB fallback path.
func mirrorLocalStorage() {
	ls := js.Global().Get("localStorage")
	if !ls.Truthy() {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	for i := 0; i < ls.Get("length").Int(); i++ {
		k := ls.Call("key", i)
		if k.Truthy() && strings.HasPrefix(k.String(), keyPrefix) {
			if v := ls.Call("getItem", k.String()); v.Truthy() {
				cache[k.String()] = v.String()
			}
		}
	}
}

// migrateLocalStorage copies every cashflux:* localStorage entry into the cache +
// IndexedDB, then removes the localStorage original so the app depends on no
// localStorage at all. Every persistence site now routes through this store, so the
// copy is authoritative; the cache holds the live value, so an interrupted migration
// can't lose data (it re-persists on the next write). Idempotent: only fills keys
// IndexedDB doesn't already have.
func migrateLocalStorage() {
	ls := js.Global().Get("localStorage")
	if !ls.Truthy() {
		return
	}
	var keys []string
	for i := 0; i < ls.Get("length").Int(); i++ {
		k := ls.Call("key", i)
		if k.Truthy() && strings.HasPrefix(k.String(), keyPrefix) {
			keys = append(keys, k.String())
		}
	}
	for _, key := range keys {
		mu.Lock()
		_, have := cache[key]
		mu.Unlock()
		if !have {
			if v := ls.Call("getItem", key); v.Truthy() {
				mu.Lock()
				cache[key] = v.String()
				mu.Unlock()
				idbPut(key, v.String())
			}
		}
		ls.Call("removeItem", key)
	}
}

func idbPut(k, v string) {
	if !db.Truthy() {
		return
	}
	defer func() { _ = recover() }()
	tx := db.Call("transaction", idbStoreName, "readwrite")
	tx.Call("objectStore", idbStoreName).Call("put", v, k)
}

func idbDelete(k string) {
	if !db.Truthy() {
		return
	}
	defer func() { _ = recover() }()
	tx := db.Call("transaction", idbStoreName, "readwrite")
	tx.Call("objectStore", idbStoreName).Call("delete", k)
}

// Get returns the value for key and whether present.
func Get(key string) (string, bool) {
	mu.Lock()
	defer mu.Unlock()
	v, ok := cache[key]
	return v, ok
}

// GetString returns the value for key, or "" when absent (localStorage-style).
func GetString(key string) string { v, _ := Get(key); return v }

// Set writes key→val to the cache and persists to IndexedDB.
func Set(key, val string) {
	mu.Lock()
	cache[key] = val
	mu.Unlock()
	idbPut(key, val)
}

// SetThen writes key→val and invokes done() once the IndexedDB transaction commits
// (or errors). Use it when the page is about to reload (e.g. a wipe): run the reload
// inside done() so the write isn't lost to the async path. It does NOT block — which
// matters because it's called from JS-invoked handlers where blocking would deadlock
// the event loop (the IDB callback could never fire). Falls back to calling done()
// immediately when IndexedDB is unavailable.
func SetThen(key, val string, done func()) {
	mu.Lock()
	cache[key] = val
	mu.Unlock()
	if !db.Truthy() {
		done()
		return
	}
	var once sync.Once
	fin := func() { once.Do(done) }
	defer func() {
		if r := recover(); r != nil {
			fin()
		}
	}()
	tx := db.Call("transaction", idbStoreName, "readwrite")
	tx.Set("oncomplete", js.FuncOf(func(js.Value, []js.Value) any { fin(); return nil }))
	tx.Set("onerror", js.FuncOf(func(js.Value, []js.Value) any { fin(); return nil }))
	tx.Set("onabort", js.FuncOf(func(js.Value, []js.Value) any { fin(); return nil }))
	tx.Call("objectStore", idbStoreName).Call("put", val, key)
}

// Remove deletes a key from the cache and IndexedDB.
func Remove(key string) {
	mu.Lock()
	delete(cache, key)
	mu.Unlock()
	idbDelete(key)
}

// RegisterJSBridge exposes the store to vendored JS (the music player, the
// widget-builder canvas shim) as window.cashfluxStore{Get,Set,Remove}, so even JS
// persistence routes through IndexedDB and the app touches no localStorage at all.
// Get is synchronous (in-memory cache); Set/Remove persist asynchronously.
func RegisterJSBridge() {
	js.Global().Set("cashfluxStoreGet", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) < 1 {
			return nil
		}
		if v, ok := Get(args[0].String()); ok {
			return v
		}
		return nil
	}))
	js.Global().Set("cashfluxStoreSet", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) >= 2 {
			Set(args[0].String(), args[1].String())
		}
		return nil
	}))
	js.Global().Set("cashfluxStoreRemove", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) >= 1 {
			Remove(args[0].String())
		}
		return nil
	}))
}

// Keys returns all cached keys (used by the wipe to enumerate).
func Keys() []string {
	mu.Lock()
	defer mu.Unlock()
	out := make([]string, 0, len(cache))
	for k := range cache {
		out = append(out, k)
	}
	return out
}
