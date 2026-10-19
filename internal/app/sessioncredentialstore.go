// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/atrest"
	"github.com/monstercameron/CashFlux/internal/keyring"
)

// Refresh credentials have an account-wide device wrap, separate from the
// active household WDK and App Lock. Opening this store never reconfigures the
// application's browserstore or imports an unscoped legacy credential.
const refreshKeyWorkspace = "auth-refresh-session"
const refreshObjectStore = "credentials"

func refreshDatabaseName(scope refreshScope) string {
	raw, _ := json.Marshal([]string{scope.ServerID, scope.AccountID})
	return "cashflux-refresh-v1:" + base64.RawURLEncoding.EncodeToString(raw)
}

type refreshRecords struct{ db js.Value }

func openRefreshStorage(scope refreshScope, create bool) (store atrest.Storage, closeStore func(), err error) {
	if err = scope.validate(); err != nil {
		return
	}
	records, err := openRefreshRecords(scope)
	if err != nil {
		return store, nil, err
	}
	closeStore = func() { records.db.Call("close") }
	store.Backend = records
	keys, err := records.Keys()
	if err != nil {
		closeStore()
		return store, nil, err
	}
	if len(keys) == 0 && !create {
		return store, closeStore, nil
	}
	keyScope := keyring.Scope{ServerID: scope.ServerID, AccountID: scope.AccountID}
	keyStore, err := atrest.NewBrowserKeyStore(keyScope, refreshKeyWorkspace)
	if err != nil {
		closeStore()
		return store, nil, err
	}
	service, err := atrest.OpenKeyService(keyStore, keyScope, refreshKeyWorkspace, len(keys) != 0)
	if err != nil {
		keyStore.Close()
		closeStore()
		return store, nil, err
	}
	store.Cipher = service
	closeStore = func() { service.Lock(); keyStore.Close(); records.db.Call("close") }
	return store, closeStore, nil
}

func refreshBrowserError(err *error) {
	if r := recover(); r != nil {
		*err = fmt.Errorf("session credential storage: %v", r)
	}
}

func openRefreshRecords(scope refreshScope) (records refreshRecords, err error) {
	defer refreshBrowserError(&err)
	idb := js.Global().Get("indexedDB")
	if !idb.Truthy() {
		return records, errors.New("session credential storage: IndexedDB is unavailable")
	}
	req := idb.Call("open", refreshDatabaseName(scope), 1)
	done := make(chan error, 1)
	upgrade := js.FuncOf(func(js.Value, []js.Value) any {
		var failure error
		func() {
			defer refreshBrowserError(&failure)
			req.Get("result").Call("createObjectStore", refreshObjectStore)
		}()
		if failure != nil {
			req.Get("transaction").Call("abort")
		}
		return nil
	})
	ok := js.FuncOf(func(js.Value, []js.Value) any { done <- nil; return nil })
	fail := js.FuncOf(func(js.Value, []js.Value) any {
		done <- fmt.Errorf("open session credential database: %s", req.Get("error").String())
		return nil
	})
	defer upgrade.Release()
	defer ok.Release()
	defer fail.Release()
	req.Set("onupgradeneeded", upgrade)
	req.Set("onsuccess", ok)
	req.Set("onerror", fail)
	if err = <-done; err == nil {
		records.db = req.Get("result")
	}
	return
}

// transaction acknowledges only the terminal transaction event. Every mutation
// requires strict durability; a browser that rejects it gets an error.
func (r refreshRecords) transaction(write bool, issue func(js.Value, func(js.Func), func(error))) (err error) {
	defer refreshBrowserError(&err)
	var callbacks []js.Func
	defer func() {
		for _, callback := range callbacks {
			callback.Release()
		}
	}()
	mode := "readonly"
	if write {
		mode = "readwrite"
	}
	tx := r.db.Call("transaction", refreshObjectStore, mode, map[string]any{"durability": "strict"})
	done := make(chan error, 1)
	var failure error
	retain := func(fn js.Func) { callbacks = append(callbacks, fn) }
	complete := js.FuncOf(func(js.Value, []js.Value) any { done <- failure; return nil })
	abort := js.FuncOf(func(js.Value, []js.Value) any {
		if failure == nil {
			failure = fmt.Errorf("session credential transaction aborted: %s", tx.Get("error").String())
		}
		done <- failure
		return nil
	})
	retain(complete)
	retain(abort)
	tx.Set("oncomplete", complete)
	tx.Set("onabort", abort)
	issue(tx.Call("objectStore", refreshObjectStore), retain, func(err error) { failure = err; tx.Call("abort") })
	return <-done
}

func (r refreshRecords) Read(key string) (value string, present bool, err error) {
	err = r.transaction(false, func(os js.Value, retain func(js.Func), fail func(error)) {
		req := os.Call("get", key)
		ok := js.FuncOf(func(js.Value, []js.Value) any {
			v := req.Get("result")
			present = !v.IsUndefined() && !v.IsNull()
			if present {
				if v.Type() != js.TypeString {
					fail(errors.New("session credential storage: invalid record type"))
					return nil
				}
				value = v.String()
			}
			return nil
		})
		retain(ok)
		req.Set("onsuccess", ok)
	})
	return
}

func (r refreshRecords) Keys() (keys []string, err error) {
	err = r.transaction(false, func(os js.Value, retain func(js.Func), _ func(error)) {
		req := os.Call("getAllKeys")
		ok := js.FuncOf(func(js.Value, []js.Value) any {
			value := req.Get("result")
			for i := 0; i < value.Length(); i++ {
				keys = append(keys, value.Index(i).String())
			}
			return nil
		})
		retain(ok)
		req.Set("onsuccess", ok)
	})
	return
}

func (r refreshRecords) Replace(key, value string, expected *string) error {
	return r.mutate(key, &value, expected)
}
func (r refreshRecords) Delete(key string, expected *string) error {
	return r.mutate(key, nil, expected)
}

func (r refreshRecords) mutate(key string, value, expected *string) error {
	return r.transaction(true, func(os js.Value, retain func(js.Func), fail func(error)) {
		req := os.Call("get", key)
		ok := js.FuncOf(func(js.Value, []js.Value) any {
			var failure error
			func() {
				defer refreshBrowserError(&failure)
				old := req.Get("result")
				present := !old.IsUndefined() && !old.IsNull()
				if (expected == nil && present) || (expected != nil && (!present || old.Type() != js.TypeString || old.String() != *expected)) {
					failure = atrest.ErrRecordChanged
					return
				}
				if value == nil {
					os.Call("delete", key)
				} else {
					os.Call("put", *value, key)
				}
			}()
			if failure != nil {
				fail(failure)
			}
			return nil
		})
		retain(ok)
		req.Set("onsuccess", ok)
	})
}

func saveRefreshCredential(scope refreshScope, token, expected string) error {
	raw, err := encodeRefreshCredential(scope, token)
	if err != nil {
		return err
	}
	store, closeStore, err := openRefreshStorage(scope, true)
	if err != nil {
		return err
	}
	defer closeStore()
	old, present, err := store.Backend.Read(scope.key())
	if err != nil {
		return err
	}
	plain := ""
	if present {
		plain, err = store.Decode(scope.key(), old)
		if err != nil {
			return err
		}
	}
	if plain != expected {
		return atrest.ErrRecordChanged
	}
	sealed, err := store.Encode(scope.key(), raw)
	if err != nil {
		return err
	}
	var prior *string
	if present {
		prior = &old
	}
	return store.Backend.Replace(scope.key(), sealed, prior)
}

func removeRefreshCredential(scope refreshScope) error {
	if err := scope.validate(); err != nil {
		return err
	}
	records, err := openRefreshRecords(scope)
	if err != nil {
		return err
	}
	defer records.db.Call("close")
	return (atrest.Storage{Backend: records}).Remove(scope.key())
}
