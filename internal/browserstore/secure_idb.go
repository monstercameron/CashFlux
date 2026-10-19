//go:build js && wasm

package browserstore

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/atrest"
)

// idbRecords never consults the plaintext cache. Every read waits for completion;
// every mutation does its comparison and write in the same strict transaction.
type idbRecords struct{}

func (idbRecords) Read(key string) (value string, present bool, err error) {
	err = readTransaction(func(os js.Value, retain func(js.Func)) {
		req := os.Call("get", key)
		fn := js.FuncOf(func(js.Value, []js.Value) any {
			v := req.Get("result")
			present = v.Type() == js.TypeString
			if present {
				value = v.String()
			}
			return nil
		})
		retain(fn)
		req.Set("onsuccess", fn)
	})
	return
}

func (idbRecords) Keys() (keys []string, err error) {
	err = readTransaction(func(os js.Value, retain func(js.Func)) {
		req := os.Call("getAllKeys")
		fn := js.FuncOf(func(js.Value, []js.Value) any {
			v := req.Get("result")
			for i := 0; i < v.Length(); i++ {
				keys = append(keys, v.Index(i).String())
			}
			return nil
		})
		retain(fn)
		req.Set("onsuccess", fn)
	})
	return
}

func readTransaction(issue func(js.Value, func(js.Func))) (err error) {
	return recordTransaction(false, func(tx js.Value, retain func(js.Func), _ func(error)) {
		issue(tx.Call("objectStore", idbStoreName), retain)
	})
}

func (idbRecords) Replace(key, value string, expected *string) error {
	return mutateRecord(key, &value, expected)
}

func (idbRecords) Delete(key string, expected *string) error { return mutateRecord(key, nil, expected) }

func mutateRecord(key string, value, expected *string) error {
	return recordTransaction(true, func(tx js.Value, retain func(js.Func), fail func(error)) {
		os := tx.Call("objectStore", idbStoreName)
		req := os.Call("get", key)
		fn := js.FuncOf(func(js.Value, []js.Value) any {
			defer func() {
				if r := recover(); r != nil {
					fail(jsPanicError("conditional record write", r))
					tx.Call("abort")
				}
			}()
			v := req.Get("result")
			present := !v.IsUndefined() && !v.IsNull()
			if (expected == nil && present) || (expected != nil && (!present || v.Type() != js.TypeString || v.String() != *expected)) {
				fail(atrest.ErrRecordChanged)
				tx.Call("abort")
				return nil
			}
			if value == nil {
				os.Call("delete", key)
			} else {
				os.Call("put", *value, key)
			}
			return nil
		})
		retain(fn)
		req.Set("onsuccess", fn)
	})
}

// transactionWaitLimit bounds missing browser completion events, including a
// transaction queued behind a stalled writer in another tab.
var transactionWaitLimit = 30 * time.Second

func recordTransaction(write bool, issue func(js.Value, func(js.Func), func(error))) error {
	return recordTransactionContext(context.Background(), write, issue)
}

func recordTransactionContext(ctx context.Context, write bool, issue func(js.Value, func(js.Func), func(error))) (err error) {
	ctx, cancel := context.WithTimeout(ctx, transactionWaitLimit)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	if !db.Truthy() {
		return errors.New("IndexedDB is unavailable; write was not persisted")
	}
	var callbacks []js.Func
	var abandoned bool
	release := func() {
		for _, fn := range callbacks {
			fn.Release()
		}
	}
	defer func() {
		if !abandoned {
			release()
		}
		if r := recover(); r != nil {
			err = jsPanicError("IndexedDB record transaction", r)
		}
	}()
	var tx js.Value
	if write {
		tx, err = openReadwriteTransaction()
		if err != nil {
			return err
		}
	} else {
		tx = db.Call("transaction", idbStoreName, "readonly")
	}
	done := make(chan error, 1)
	var once sync.Once
	var failure error
	settle := func(err error) { once.Do(func() { done <- err }) }
	retain := func(fn js.Func) { callbacks = append(callbacks, fn) }
	bind := func(event string, fn func(js.Value, []js.Value) any) { f := js.FuncOf(fn); retain(f); tx.Set(event, f) }
	bind("oncomplete", func(js.Value, []js.Value) any { settle(failure); return nil })
	bind("onabort", func(js.Value, []js.Value) any {
		if failure != nil {
			settle(failure)
		} else {
			settle(transactionFailure(tx, "abort"))
		}
		return nil
	})
	// Wait for the final abort event; a request error is not a durable result.
	bind("onerror", func(js.Value, []js.Value) any {
		if failure == nil {
			failure = transactionFailure(tx, "error")
		}
		return nil
	})
	issue(tx, retain, func(err error) { failure = err })
	select {
	case result := <-done:
		return result
	case <-ctx.Done():
		err := fmt.Errorf("IndexedDB transaction did not settle: %w", ctx.Err())
		// An unknown write outcome must never grant autosave authority. Abort also
		// releases transactions queued behind this one. A late complete is NOT an
		// acknowledgement; the page stays paused until it reopens storage.
		if write {
			secureInitErr = err
		}
		slog.Error("protected storage transaction paused", "write", write, "error", err)
		abandoned = true
		func() { defer func() { _ = recover() }(); tx.Call("abort") }()
		// Keep callbacks alive until the terminal event, even when abort delivery is
		// delayed. Releasing them on timeout would make late browser events call a
		// released Go function. If the browser never settles, retain this small set.
		go func() { <-done; release() }()
		return err
	}
}
