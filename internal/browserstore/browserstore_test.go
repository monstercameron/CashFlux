// SPDX-License-Identifier: MIT

//go:build js && wasm

package browserstore

import (
	"strings"
	"syscall/js"
	"testing"
	"time"
)

// These exercise the synchronous in-memory cache layer (Get/Set/Remove/Keys/
// GetString) without IndexedDB: with db unset, the async idbPut/idbDelete no-op, so
// the cache is the whole behavior. Run via the wasm test lane (GOOS=js GOARCH=wasm).

func reset() {
	mu.Lock()
	cache = map[string]string{}
	mu.Unlock()
	db = js.Value{}
	durable = false
	ready = false
}

func installMockDB(t *testing.T, mode string) {
	t.Helper()
	factory := js.Global().Get("Function").New("mode", `return {transaction:function(_store,_mode,options){
  if (mode === "strict-unsupported" && options) throw new TypeError("transaction options unsupported");
  if (mode === "open-failure") throw Object.assign(new Error("closed"), {name:"InvalidStateError"});
  var tx = {error:null, objectStore:function(){return {put:function(){}, delete:function(){}};}};
  setTimeout(function(){
    if (mode === "abort") { tx.error = {name:"QuotaExceededError"}; if (tx.onabort) tx.onabort(); }
    else if (tx.oncomplete) tx.oncomplete();
  }, 0);
  return tx;
}}`)
	db = factory.Invoke(mode)
	durable = true
	t.Cleanup(func() {
		db = js.Value{}
		durable = false
	})
}

func waitResult(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for IndexedDB transaction")
		return nil
	}
}

func TestCacheSetGetRemove(t *testing.T) {
	reset()
	if _, ok := Get("cashflux:x"); ok {
		t.Fatal("expected miss on empty cache")
	}
	if GetString("cashflux:x") != "" {
		t.Fatal("GetString should be empty on miss")
	}
	Set("cashflux:x", "1")
	if v, ok := Get("cashflux:x"); !ok || v != "1" {
		t.Fatalf("Get after Set = %q ok=%v", v, ok)
	}
	if GetString("cashflux:x") != "1" {
		t.Fatal("GetString after Set")
	}
	// Overwrite.
	Set("cashflux:x", "2")
	if GetString("cashflux:x") != "2" {
		t.Fatal("overwrite failed")
	}
	Remove("cashflux:x")
	if _, ok := Get("cashflux:x"); ok {
		t.Fatal("Remove did not delete")
	}
}

func TestCacheKeys(t *testing.T) {
	reset()
	Set("cashflux:a", "1")
	Set("cashflux:b", "2")
	Set("cashflux:c", "3")
	keys := Keys()
	if len(keys) != 3 {
		t.Fatalf("Keys len = %d, want 3", len(keys))
	}
	seen := map[string]bool{}
	for _, k := range keys {
		seen[k] = true
	}
	for _, want := range []string{"cashflux:a", "cashflux:b", "cashflux:c"} {
		if !seen[want] {
			t.Errorf("Keys missing %q", want)
		}
	}
}

func TestSetThenWithoutDBUpdatesCacheBeforeCallback(t *testing.T) {
	reset()
	callbackValue := ""
	SetThen("cashflux:k", "v", func() { callbackValue = GetString("cashflux:k") })
	if got := GetString("cashflux:k"); got != "v" {
		t.Fatalf("GetString immediately after SetThen = %q, want v", got)
	}
	if callbackValue != "v" {
		t.Fatalf("GetString in fallback callback = %q, want v", callbackValue)
	}
}

func TestSetThenWithDBUpdatesCacheBeforeCommitAndCallback(t *testing.T) {
	reset()
	installMockDB(t, "success")
	callbackValue := make(chan string, 1)
	SetThen("cashflux:k", "v", func() { callbackValue <- GetString("cashflux:k") })
	if got := GetString("cashflux:k"); got != "v" {
		t.Fatalf("GetString immediately after SetThen = %q, want v", got)
	}
	select {
	case got := <-callbackValue:
		if got != "v" {
			t.Fatalf("GetString in IndexedDB callback = %q, want v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SetThen callback")
	}
}

func TestSetDurableUpdatesCacheOnlyAfterCommit(t *testing.T) {
	reset()
	installMockDB(t, "success")
	cache["cashflux:durable"] = "old"
	result := make(chan error, 1)
	SetDurable("cashflux:durable", "new", func(err error) { result <- err })
	if got := GetString("cashflux:durable"); got != "old" {
		t.Fatalf("cache changed before transaction completed: %q", got)
	}
	if err := waitResult(t, result); err != nil {
		t.Fatalf("SetDurable error = %v", err)
	}
	if got := GetString("cashflux:durable"); got != "new" {
		t.Fatalf("cache after commit = %q, want new", got)
	}
}

func TestDurableAbortPreservesCacheAndNamesDOMError(t *testing.T) {
	reset()
	installMockDB(t, "abort")
	cache["cashflux:durable"] = "old"
	result := make(chan error, 1)
	SetDurable("cashflux:durable", "new", func(err error) { result <- err })
	if err := waitResult(t, result); err == nil || !strings.Contains(err.Error(), "QuotaExceededError") {
		t.Fatalf("abort error = %v, want QuotaExceededError", err)
	}
	if got := GetString("cashflux:durable"); got != "old" {
		t.Fatalf("failed transaction changed cache to %q", got)
	}
}

func TestClosedDatabaseReportsDOMExceptionName(t *testing.T) {
	reset()
	installMockDB(t, "open-failure")
	result := make(chan error, 1)
	SetDurable("cashflux:durable", "value", func(err error) { result <- err })
	if err := waitResult(t, result); err == nil || !strings.Contains(err.Error(), "InvalidStateError") {
		t.Fatalf("closed database error = %v, want InvalidStateError", err)
	}
	if _, ok := Get("cashflux:durable"); ok {
		t.Fatal("failed transaction changed the cache")
	}
}

func TestRemoveDurableChangesCacheOnlyAfterCommit(t *testing.T) {
	reset()
	installMockDB(t, "success")
	cache["cashflux:durable"] = "present"
	result := make(chan error, 1)
	RemoveDurable("cashflux:durable", func(err error) { result <- err })
	if got := GetString("cashflux:durable"); got != "present" {
		t.Fatalf("cache changed before transaction completed: %q", got)
	}
	if err := waitResult(t, result); err != nil {
		t.Fatalf("RemoveDurable error = %v", err)
	}
	if _, ok := Get("cashflux:durable"); ok {
		t.Fatal("cache entry remains after committed delete")
	}
}

func TestStrictOptionsUnsupportedFallsBackSilently(t *testing.T) {
	reset()
	installMockDB(t, "strict-unsupported")
	result := make(chan error, 1)
	SetDurable("cashflux:durable", "value", func(err error) { result <- err })
	if err := waitResult(t, result); err != nil {
		t.Fatalf("fallback transaction error = %v", err)
	}
	if got := GetString("cashflux:durable"); got != "value" {
		t.Fatalf("cache = %q, want committed value", got)
	}
}

func TestDurableFallbackReportsFailureWithoutChangingCache(t *testing.T) {
	reset()
	cache["cashflux:durable"] = "old"
	if Durable() {
		t.Fatal("fallback must not report durable storage")
	}
	result := make(chan error, 1)
	SetDurable("cashflux:durable", "new", func(err error) { result <- err })
	if err := waitResult(t, result); err == nil || !strings.Contains(err.Error(), "IndexedDB is unavailable") {
		t.Fatalf("fallback error = %v, want unavailable error", err)
	}
	if got := GetString("cashflux:durable"); got != "old" {
		t.Fatalf("fallback changed cache to %q", got)
	}
	RemoveDurable("cashflux:durable", func(err error) { result <- err })
	if err := waitResult(t, result); err == nil {
		t.Fatal("fallback remove reported success")
	}
}

func TestInitWithoutIndexedDBReportsNonDurable(t *testing.T) {
	reset()
	Init()
	if Durable() {
		t.Fatal("Init without IndexedDB must report non-durable fallback")
	}
}
