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
	"errors"
	"fmt"
	"log/slog"
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
	mu      sync.Mutex
	cache   = map[string]string{}
	db      js.Value
	ready   bool
	durable bool
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
	// Start listening before the load, so a write another tab makes while this
	// one is still opening IndexedDB is not missed.
	StartCrossTab()
	idb := js.Global().Get("indexedDB")
	if !idb.Truthy() {
		durable = false
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
		durable = false
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
				durable = false
				mirrorLocalStorage()
				finish()
			}
		}()
		db = req.Get("result")
		// Guard: if the expected object store is absent, don't transact (would throw).
		if !db.Get("objectStoreNames").Call("contains", idbStoreName).Bool() {
			db = js.Value{}
			durable = false
			mirrorLocalStorage()
			finish()
			return nil
		}
		durable = true
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
			durable = false
			mirrorLocalStorage()
			finish()
			return nil
		}))
		return nil
	}))
	<-done
}

// Durable reports whether IndexedDB is available as the persistence backend.
// It is false after Init falls back to the in-memory/localStorage mirror, where
// writes cannot be acknowledged as durable.
func Durable() bool { return durable }

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
	publish(key)
}

// SetThen updates the in-memory cache and publishes key→val immediately, then
// invokes done() after the IndexedDB transaction completes or fails. The cache
// remains readable immediately even when IndexedDB is unavailable. Use
// SetDurable when the caller needs to distinguish durable success from failure.
// It does NOT block, which matters because calls from JS-invoked handlers would
// deadlock the event loop if they waited for the IndexedDB callback.
func SetThen(key, val string, done func()) {
	mu.Lock()
	cache[key] = val
	mu.Unlock()
	publish(key)

	SetDurable(key, val, func(err error) {
		if err != nil {
			slog.Error("browserstore write failed", "key", key, "error", err)
		}
		if done != nil {
			done()
		}
	})
}

// SetDurable writes key→val in a strict-durability IndexedDB transaction. The
// in-memory cache is updated only after the transaction's complete event. If
// IndexedDB is unavailable, or the transaction errors or aborts, the cache is
// left unchanged and done receives an error. The callback is asynchronous when
// IndexedDB is available and never blocks the browser event loop.
func SetDurable(key, val string, done func(error)) {
	writeDurable(key, done, func(os js.Value) { os.Call("put", val, key) }, func() { cache[key] = val })
}

// RemoveDurable removes key in a strict-durability IndexedDB transaction. The
// in-memory cache is updated only after the transaction's complete event. If
// IndexedDB is unavailable, or the transaction errors or aborts, the cache is
// left unchanged and done receives an error.
func RemoveDurable(key string, done func(error)) {
	writeDurable(key, done, func(os js.Value) { os.Call("delete", key) }, func() { delete(cache, key) })
}

func writeDurable(key string, done func(error), issue func(js.Value), applyCache func()) {
	finish := func(err error) {
		if done != nil {
			done(err)
		}
	}
	if !db.Truthy() {
		finish(errors.New("IndexedDB is unavailable; write was not persisted"))
		return
	}
	tx, err := openReadwriteTransaction()
	if err != nil {
		finish(err)
		return
	}
	var once sync.Once
	settle := func(err error) {
		once.Do(func() {
			if err == nil {
				mu.Lock()
				applyCache()
				mu.Unlock()
				publish(key)
			}
			finish(err)
		})
	}
	tx.Set("oncomplete", js.FuncOf(func(js.Value, []js.Value) any { settle(nil); return nil }))
	tx.Set("onerror", js.FuncOf(func(js.Value, []js.Value) any { settle(transactionFailure(tx, "error")); return nil }))
	tx.Set("onabort", js.FuncOf(func(js.Value, []js.Value) any { settle(transactionFailure(tx, "abort")); return nil }))
	defer func() {
		if recovered := recover(); recovered != nil {
			settle(jsPanicError("IndexedDB request failed", recovered))
		}
	}()
	issue(tx.Call("objectStore", idbStoreName))
}

func openReadwriteTransaction() (tx js.Value, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = jsPanicError("open IndexedDB transaction", recovered)
		}
	}()
	options := js.Global().Get("Object").New()
	options.Set("durability", "strict")
	defer func() {
		if recovered := recover(); recovered != nil {
			if isOptionError(recovered) {
				tx = db.Call("transaction", idbStoreName, "readwrite")
				err = nil
			} else {
				err = jsPanicError("open IndexedDB transaction", recovered)
			}
		}
	}()
	tx = db.Call("transaction", idbStoreName, "readwrite", options)
	return tx, nil
}

func isOptionError(recovered any) bool {
	name := jsExceptionName(recovered)
	return name == "TypeError" || name == "NotSupportedError"
}

func transactionFailure(tx js.Value, event string) error {
	if tx.Truthy() {
		errValue := tx.Get("error")
		if errValue.Truthy() {
			name := errValue.Get("name")
			if name.Type() == js.TypeString && name.String() != "" {
				return fmt.Errorf("IndexedDB transaction %s: %s", event, name.String())
			}
		}
	}
	return fmt.Errorf("IndexedDB transaction %s", event)
}

func jsPanicError(context string, recovered any) error {
	name := jsExceptionName(recovered)
	if name == "" {
		return fmt.Errorf("%s: %v", context, recovered)
	}
	return fmt.Errorf("%s: %s", context, name)
}

func jsExceptionName(recovered any) string {
	var value js.Value
	switch v := recovered.(type) {
	case js.Error:
		value = v.Value
	case js.Value:
		value = v
	default:
		return ""
	}
	if value.Truthy() {
		name := value.Get("name")
		if name.Type() == js.TypeString {
			return name.String()
		}
	}
	return ""
}

// Remove deletes a key from the cache and IndexedDB.
func Remove(key string) {
	mu.Lock()
	delete(cache, key)
	mu.Unlock()
	idbDelete(key)
	publish(key)
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
