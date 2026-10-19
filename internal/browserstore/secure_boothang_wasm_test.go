// SPDX-License-Identifier: MIT

//go:build js && wasm

package browserstore

import (
	"context"
	"errors"
	"runtime"
	"syscall/js"
	"testing"
	"time"
)

func bootResult(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(3 * time.Second):
		stack := make([]byte, 1<<20)
		t.Fatalf("BOOT storage stalled\n%s", stack[:runtime.Stack(stack, true)])
		return nil
	}
}

func TestBOOTCompletionCanWaitForQueuedWriteAndBarrier(t *testing.T) {
	initFixture(t)
	done := make(chan error, 1)
	SetSecure("cashflux:boot-first", "first", func(err error) {
		if err != nil {
			done <- err
			return
		}
		nested := make(chan error, 1)
		SetSecure("cashflux:boot-second", "second", func(err error) { nested <- err })
		Barrier()
		done <- <-nested
	})
	RemoveSecure("cashflux:boot-absent", nil)
	if err := bootResult(t, done); err != nil {
		t.Fatal(err)
	}
	value, _, err := GetSecure("cashflux:boot-second")
	if err != nil || value != "second" {
		t.Fatalf("nested write missing: %q %v", value, err)
	}
}

func TestBOOTTransactionTimeoutAbortsAndPreservesRecords(t *testing.T) {
	for _, write := range []bool{false, true} {
		t.Run(map[bool]string{false: "snapshot", true: "write"}[write], func(t *testing.T) {
			initFixture(t)
			put(t, "cashflux:boot-kept", "original")
			original := db
			// Stall a real shim transaction, including transactions queued behind it.
			// The production deadline must abort it and release the database queue.
			db = js.Global().Get("Function").New("db", `return new Proxy(db, {get(target, key) {
    if (key !== "transaction") return Reflect.get(target,key);
    return (...args) => { const tx=target.transaction(...args); tx._step=()=>{}; return tx; };
   }});`).Invoke(original)
			oldLimit := transactionWaitLimit
			transactionWaitLimit = 50 * time.Millisecond
			err := recordTransaction(write, func(tx js.Value, _ func(js.Func), _ func(error)) {
				os := tx.Call("objectStore", idbStoreName)
				if write {
					os.Call("put", "must not commit", "cashflux:boot-kept")
				} else {
					os.Call("get", "cashflux:boot-kept")
				}
			})
			transactionWaitLimit = oldLimit
			db = original
			if !errors.Is(err, context.DeadlineExceeded) || (write && !errors.Is(InitError(), context.DeadlineExceeded)) {
				t.Fatalf("unbounded or unpaused storage: %v / %v", err, InitError())
			}
			// Let the abort callback settle, then check durable bytes using a fresh read.
			secureInitErr = nil
			value, _, readErr := GetSecure("cashflux:boot-kept")
			if readErr != nil || value != "original" {
				t.Fatalf("timeout changed durable source: %q %v", value, readErr)
			}
		})
	}
}

func TestBOOTSnapshotCanceledBeforeRead(t *testing.T) {
	initFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	snapshot, err := ReadSecureSnapshotContext(ctx, []string{"cashflux:dataset"}, nil)
	if !errors.Is(err, context.Canceled) || len(snapshot.Values) != 0 {
		t.Fatalf("canceled read published snapshot: %+v %v", snapshot, err)
	}
}

func TestBOOTCanceledQueuedRepairNeverWrites(t *testing.T) {
	initFixture(t)
	put(t, "cashflux:dataset", "original")
	snapshot, err := ReadSecureSnapshot([]string{"cashflux:dataset"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	blocked := make(chan struct{})
	started := make(chan struct{})
	enqueue(func() { close(started); <-blocked })
	<-started
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	SetSecureSnapshotBatchContext(ctx, map[string]string{"cashflux:dataset": "late repair"}, snapshot, []string{"cashflux:dataset"}, func(err error) { done <- err })
	cancel()
	close(blocked)
	if err := bootResult(t, done); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled repair = %v", err)
	}
	value, _, err := GetSecure("cashflux:dataset")
	if err != nil || value != "original" {
		t.Fatalf("late repair changed source: %q %v", value, err)
	}
}

func TestBOOTOpenDeadlineDoesNotSeedReplacement(t *testing.T) {
	fixture(t)
	oldLimit := openWaitLimit
	openWaitLimit = 50 * time.Millisecond
	defer func() { openWaitLimit = oldLimit }()
	js.Global().Set("indexedDB", js.Global().Get("Function").New("return {open:()=>({})}").Invoke())
	Init()
	if InitError() == nil || Durable() || SecureKeyState() != nil || len(cache) != 0 {
		t.Fatal("stalled database open was mistaken for an empty usable profile")
	}
}

func TestBOOTKeyServiceDeadlineDoesNotSeedReplacement(t *testing.T) {
	fixture(t)
	oldLimit := secureInitWaitLimit
	secureInitWaitLimit = 50 * time.Millisecond
	defer func() { secureInitWaitLimit = oldLimit }()
	factory := js.Global().Get("indexedDB")
	js.Global().Set("indexedDB", js.Global().Get("Function").New("factory", `return {open(name,...args) {
  if(name.startsWith("cashflux-atrest-keys-v1:")) return {};
  return factory.open(name,...args);
 }};`).Invoke(factory))
	Init()
	if InitError() == nil || SecureKeyState() != nil || len(cache) != 0 {
		t.Fatal("stalled key open was mistaken for an empty usable profile")
	}
}

func TestBOOTLateCompletionAfterDeadline(t *testing.T) {
	initFixture(t)
	original := db
	db = js.Global().Get("Function").New("db", `return new Proxy(db,{get(target,key){
  if(key!=="transaction") return Reflect.get(target,key);
  return (...args)=>{
   const tx=target.transaction(...args);
   globalThis.__bootLateTransaction=tx;
   Object.defineProperty(tx,"oncomplete",{set(cb){this._lateComplete=cb},get(){return null}});
   return tx;
  };
 }});`).Invoke(original)
	oldLimit := transactionWaitLimit
	transactionWaitLimit = 50 * time.Millisecond
	err := recordTransaction(false, func(tx js.Value, _ func(js.Func), _ func(error)) {
		tx.Call("objectStore", idbStoreName).Call("get", "cashflux:dataset")
	})
	transactionWaitLimit = oldLimit
	db = original
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("missing complete = %v", err)
	}
	tx := js.Global().Get("__bootLateTransaction")
	tx.Get("_lateComplete").Invoke()
	js.Global().Delete("__bootLateTransaction")
	// A late terminal event must reach a live callback, rather than a released
	// js.Func; a following ordinary read must still complete.
	if _, _, err := GetSecure("cashflux:dataset"); err != nil {
		t.Fatal(err)
	}
}
