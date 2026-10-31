// SPDX-License-Identifier: MIT

//go:build js && wasm

package artifactstore

import (
	"bytes"
	"fmt"
	"syscall/js"
)

const (
	idbName      = "cashflux"
	idbStoreName = "artifacts"
	idbVersion   = 1
)

// IDBStore is an IndexedDB-backed blob store. Create one with OpenIDB; it is
// safe to keep as a long-lived singleton. Falls back to ErrUnavailable when
// IndexedDB is not accessible in the current environment.
type IDBStore struct {
	db js.Value // IDBDatabase
}

// OpenIDB opens (or upgrades) the CashFlux IndexedDB and returns a ready Store.
// The call blocks until the open request settles. Returns ErrUnavailable if
// IndexedDB is not present in the global scope.
func OpenIDB() (*IDBStore, error) {
	idb := js.Global().Get("indexedDB")
	if idb.IsUndefined() || idb.IsNull() {
		return nil, ErrUnavailable
	}

	type result struct {
		db  js.Value
		err error
	}
	ch := make(chan result, 1)

	req := idb.Call("open", idbName, idbVersion)

	var onUpgrade, onSuccess, onError js.Func

	onUpgrade = js.FuncOf(func(_ js.Value, args []js.Value) any {
		defer onUpgrade.Release()
		event := args[0]
		target := event.Get("target")
		upgradeDB := target.Get("result")
		names := upgradeDB.Get("objectStoreNames")
		contains := names.Call("contains", idbStoreName)
		if !contains.Bool() {
			upgradeDB.Call("createObjectStore", idbStoreName)
		}
		return nil
	})

	onSuccess = js.FuncOf(func(_ js.Value, args []js.Value) any {
		defer onSuccess.Release()
		defer onError.Release()
		event := args[0]
		ch <- result{db: event.Get("target").Get("result")}
		return nil
	})

	onError = js.FuncOf(func(_ js.Value, args []js.Value) any {
		defer onSuccess.Release()
		defer onError.Release()
		event := args[0]
		msg := event.Get("target").Get("error").Get("message").String()
		ch <- result{err: fmt.Errorf("artifactstore: idb open: %s", msg)}
		return nil
	})

	req.Set("onupgradeneeded", onUpgrade)
	req.Set("onsuccess", onSuccess)
	req.Set("onerror", onError)

	r := <-ch
	if r.err != nil {
		return nil, r.err
	}
	return &IDBStore{db: r.db}, nil
}

// Put stores mime + data for id in the "artifacts" object store.
func (s *IDBStore) Put(id string, mime string, data []byte) error {
	tx := s.db.Call("transaction", idbStoreName, "readwrite")
	os := tx.Call("objectStore", idbStoreName)

	// Build a JS object {mime, data: Uint8Array}.
	jsObj := js.Global().Get("Object").New()
	jsObj.Set("mime", mime)
	buf := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(buf, data)
	jsObj.Set("data", buf)

	ch := make(chan error, 1)

	var onComplete, onAbort js.Func
	onComplete = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		ch <- nil
		return nil
	})
	onAbort = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		ch <- fmt.Errorf("artifactstore: idb put %s aborted: %s", id, tx.Get("error"))
		return nil
	})
	tx.Set("oncomplete", onComplete)
	tx.Set("onabort", onAbort)
	os.Call("put", jsObj, id)
	err := <-ch
	onComplete.Release()
	onAbort.Release()
	return err
}

// Get retrieves the blob for id. Returns ok=false when not found.
func (s *IDBStore) Get(id string) (mime string, data []byte, ok bool, err error) {
	tx := s.db.Call("transaction", idbStoreName, "readonly")
	os := tx.Call("objectStore", idbStoreName)

	type result struct {
		mime string
		data []byte
		ok   bool
		err  error
	}
	ch := make(chan result, 1)

	var onSuccess, onError js.Func
	onSuccess = js.FuncOf(func(_ js.Value, args []js.Value) any {
		defer onSuccess.Release()
		defer onError.Release()
		defer func() {
			if recovered := recover(); recovered != nil {
				ch <- result{err: fmt.Errorf("artifactstore: idb get %s has an invalid record: %v", id, recovered)}
			}
		}()
		event := args[0]
		val := event.Get("target").Get("result")
		if val.IsUndefined() || val.IsNull() {
			ch <- result{ok: false}
			return nil
		}
		m := val.Get("mime").String()
		jsData := val.Get("data")
		if !jsData.InstanceOf(js.Global().Get("Uint8Array")) {
			ch <- result{err: fmt.Errorf("artifactstore: idb get %s has invalid bytes", id)}
			return nil
		}
		buf := make([]byte, jsData.Get("byteLength").Int())
		if copied := js.CopyBytesToGo(buf, jsData); copied != len(buf) {
			ch <- result{err: fmt.Errorf("artifactstore: idb get %s copied %d of %d bytes", id, copied, len(buf))}
			return nil
		}
		ch <- result{mime: m, data: buf, ok: true}
		return nil
	})
	onError = js.FuncOf(func(_ js.Value, args []js.Value) any {
		defer onSuccess.Release()
		defer onError.Release()
		event := args[0]
		msg := event.Get("target").Get("error").Get("message").String()
		ch <- result{err: fmt.Errorf("artifactstore: idb get %s: %s", id, msg)}
		return nil
	})

	req := os.Call("get", id)
	req.Set("onsuccess", onSuccess)
	req.Set("onerror", onError)

	r := <-ch
	return r.mime, r.data, r.ok, r.err
}

// Keys lists every artifact key, including records no dataset references.
func (s *IDBStore) Keys() ([]string, error) {
	tx := s.db.Call("transaction", idbStoreName, "readonly")
	req := tx.Call("objectStore", idbStoreName).Call("getAllKeys")
	type result struct {
		keys []string
		err  error
	}
	ch := make(chan result, 1)
	var onSuccess, onError js.Func
	onSuccess = js.FuncOf(func(_ js.Value, args []js.Value) any {
		array := args[0].Get("target").Get("result")
		keys := make([]string, 0, array.Length())
		for i := 0; i < array.Length(); i++ {
			key := array.Index(i)
			if key.Type() != js.TypeString {
				ch <- result{err: fmt.Errorf("artifactstore: non-string artifact key at index %d", i)}
				return nil
			}
			keys = append(keys, key.String())
		}
		ch <- result{keys: keys}
		return nil
	})
	onError = js.FuncOf(func(_ js.Value, args []js.Value) any {
		ch <- result{err: fmt.Errorf("artifactstore: list keys: %s", args[0].Get("target").Get("error"))}
		return nil
	})
	req.Set("onsuccess", onSuccess)
	req.Set("onerror", onError)
	r := <-ch
	onSuccess.Release()
	onError.Release()
	return r.keys, r.err
}

// ReplaceIfUnchanged atomically protects a legacy record without overwriting
// a newer value that another tab stored after the migration read it.
func (s *IDBStore) ReplaceIfUnchanged(id, oldMIME string, oldData []byte, newMIME string, newData []byte) (bool, error) {
	tx := s.db.Call("transaction", idbStoreName, "readwrite")
	os := tx.Call("objectStore", idbStoreName)
	ch := make(chan error, 1)
	replaced := false
	var onRead, onComplete, onAbort js.Func
	onRead = js.FuncOf(func(_ js.Value, args []js.Value) any {
		val := args[0].Get("target").Get("result")
		if val.IsUndefined() || val.IsNull() || val.Get("mime").Type() != js.TypeString || val.Get("mime").String() != oldMIME {
			return nil
		}
		current := val.Get("data")
		if !current.InstanceOf(js.Global().Get("Uint8Array")) || current.Get("byteLength").Int() != len(oldData) {
			return nil
		}
		buf := make([]byte, len(oldData))
		if js.CopyBytesToGo(buf, current) != len(buf) || !bytes.Equal(buf, oldData) {
			return nil
		}
		obj := js.Global().Get("Object").New()
		obj.Set("mime", newMIME)
		sealed := js.Global().Get("Uint8Array").New(len(newData))
		js.CopyBytesToJS(sealed, newData)
		obj.Set("data", sealed)
		os.Call("put", obj, id)
		replaced = true
		return nil
	})
	onComplete = js.FuncOf(func(_ js.Value, _ []js.Value) any { ch <- nil; return nil })
	onAbort = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		ch <- fmt.Errorf("artifactstore: conditional put %s aborted: %s", id, tx.Get("error"))
		return nil
	})
	tx.Set("oncomplete", onComplete)
	tx.Set("onabort", onAbort)
	os.Call("get", id).Set("onsuccess", onRead)
	err := <-ch
	onRead.Release()
	onComplete.Release()
	onAbort.Release()
	if err != nil {
		return false, err
	}
	return replaced, nil
}

// Delete removes the blob for id. Not an error if absent.
func (s *IDBStore) Delete(id string) error {
	tx := s.db.Call("transaction", idbStoreName, "readwrite")
	os := tx.Call("objectStore", idbStoreName)

	ch := make(chan error, 1)

	var onComplete, onAbort js.Func
	onComplete = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		ch <- nil
		return nil
	})
	onAbort = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		ch <- fmt.Errorf("artifactstore: idb delete %s aborted: %s", id, tx.Get("error"))
		return nil
	})
	tx.Set("oncomplete", onComplete)
	tx.Set("onabort", onAbort)
	os.Call("delete", id)
	err := <-ch
	onComplete.Release()
	onAbort.Release()
	return err
}

// Usage queries navigator.storage.estimate() for bytes used. Returns 0, nil
// when the Storage API is unavailable (best-effort).
func (s *IDBStore) Usage() (int64, error) {
	nav := js.Global().Get("navigator")
	if nav.IsUndefined() {
		return 0, nil
	}
	storage := nav.Get("storage")
	if storage.IsUndefined() || storage.IsNull() {
		return 0, nil
	}
	estimateFn := storage.Get("estimate")
	if estimateFn.IsUndefined() {
		return 0, nil
	}

	type result struct {
		usage int64
		err   error
	}
	ch := make(chan result, 1)

	promise := storage.Call("estimate")
	var onFulfill, onReject js.Func
	onFulfill = js.FuncOf(func(_ js.Value, args []js.Value) any {
		defer onFulfill.Release()
		defer onReject.Release()
		est := args[0]
		usage := est.Get("usage").Int()
		ch <- result{usage: int64(usage)}
		return nil
	})
	onReject = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		defer onFulfill.Release()
		defer onReject.Release()
		ch <- result{err: fmt.Errorf("artifactstore: storage.estimate rejected")}
		return nil
	})
	promise.Call("then", onFulfill).Call("catch", onReject)

	r := <-ch
	return r.usage, r.err
}
