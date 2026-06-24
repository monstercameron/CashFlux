// SPDX-License-Identifier: MIT

//go:build js && wasm

package browserstore

import "testing"

// These exercise the synchronous in-memory cache layer (Get/Set/Remove/Keys/
// GetString) without IndexedDB: with db unset, the async idbPut/idbDelete no-op, so
// the cache is the whole behavior. Run via the wasm test lane (GOOS=js GOARCH=wasm).

func reset() {
	mu.Lock()
	cache = map[string]string{}
	mu.Unlock()
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

func TestSetThenWithoutDBCallsBack(t *testing.T) {
	reset()
	called := false
	SetThen("cashflux:k", "v", func() { called = true })
	if !called {
		t.Fatal("SetThen must call done() synchronously when IndexedDB is unavailable")
	}
	if GetString("cashflux:k") != "v" {
		t.Fatal("SetThen must still update the cache")
	}
}
