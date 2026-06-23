//go:build js && wasm

package artifactstore

import (
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

	var onSuccess, onError js.Func
	onSuccess = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		defer onSuccess.Release()
		defer onError.Release()
		ch <- nil
		return nil
	})
	onError = js.FuncOf(func(_ js.Value, args []js.Value) any {
		defer onSuccess.Release()
		defer onError.Release()
		event := args[0]
		msg := event.Get("target").Get("error").Get("message").String()
		ch <- fmt.Errorf("artifactstore: idb put %s: %s", id, msg)
		return nil
	})

	req := os.Call("put", jsObj, id)
	req.Set("onsuccess", onSuccess)
	req.Set("onerror", onError)

	return <-ch
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
		event := args[0]
		val := event.Get("target").Get("result")
		if val.IsUndefined() || val.IsNull() {
			ch <- result{ok: false}
			return nil
		}
		m := val.Get("mime").String()
		jsData := val.Get("data")
		buf := make([]byte, jsData.Get("byteLength").Int())
		js.CopyBytesToGo(buf, jsData)
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

// Delete removes the blob for id. Not an error if absent.
func (s *IDBStore) Delete(id string) error {
	tx := s.db.Call("transaction", idbStoreName, "readwrite")
	os := tx.Call("objectStore", idbStoreName)

	ch := make(chan error, 1)

	var onSuccess, onError js.Func
	onSuccess = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		defer onSuccess.Release()
		defer onError.Release()
		ch <- nil
		return nil
	})
	onError = js.FuncOf(func(_ js.Value, args []js.Value) any {
		defer onSuccess.Release()
		defer onError.Release()
		event := args[0]
		msg := event.Get("target").Get("error").Get("message").String()
		ch <- fmt.Errorf("artifactstore: idb delete %s: %s", id, msg)
		return nil
	})

	req := os.Call("delete", id)
	req.Set("onsuccess", onSuccess)
	req.Set("onerror", onError)

	return <-ch
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
