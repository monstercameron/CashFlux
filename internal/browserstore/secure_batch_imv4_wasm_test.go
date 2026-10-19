//go:build js && wasm

package browserstore

import (
	"errors"
	"fmt"
	"strings"
	"syscall/js"
	"testing"

	"github.com/monstercameron/CashFlux/internal/atrest"
)

func TestIMV4SecureBatchUsesTwoRecordTransactionsAndOneNotification(t *testing.T) {
	initFixture(t)
	oldChannel := channel
	defer func() { channel = oldChannel }()
	messages := 0
	post := js.FuncOf(func(_ js.Value, args []js.Value) any {
		messages++
		keys := args[0].Get("keys")
		if keys.Length() != 30 {
			t.Errorf("notification keys=%d", keys.Length())
		}
		for i := 0; i < keys.Length(); i++ {
			if value := GetString(keys.Index(i).String()); value != "sensitive import row" {
				t.Errorf("published before the complete cache update: %q", value)
			}
		}
		encoded := js.Global().Get("JSON").Call("stringify", args[0]).String()
		if strings.Contains(encoded, "sensitive import row") {
			t.Error("notification exposed plaintext")
		}
		return nil
	})
	defer post.Release()
	channel = js.Global().Get("Object").New()
	channel.Set("postMessage", post)
	values := make(map[string]string, 30)
	for i := 0; i < 30; i++ {
		values[fmt.Sprintf("cashflux:dataset:record:imv4:%d", i)] = "sensitive import row"
	}
	control := js.Global().Get("__fakeIDB")
	before := control.Call("transactionCount").Int()
	done := make(chan error, 1)
	SetSecureBatch(values, nil, func(err error) { done <- err })
	if err := waitResult(t, done); err != nil {
		t.Fatal(err)
	}
	// KeyService re-reads key state before each seal; those safety reads are
	// additional to the two record transactions and must never be bypassed.
	if got := control.Call("transactionCount").Int() - before; got != 2+len(values) {
		t.Fatalf("transactions=%d, want two record transactions plus %d key-state checks", got, len(values))
	}
	if messages != 1 {
		t.Fatalf("notifications=%d", messages)
	}
}

func TestIMV4SecureBatchFailureRetainsCacheAndGeneration(t *testing.T) {
	for _, failure := range []string{"generation", "strict abort"} {
		t.Run(failure, func(t *testing.T) {
			initFixture(t)
			const generationKey = "cashflux:dataset:gen"
			const rowKey = "cashflux:dataset:record:imv4:row"
			put(t, generationKey, "original")
			put(t, rowKey, "original row")
			expected := "original"
			if failure == "generation" {
				expected = "stale"
			} else {
				js.Global().Get("__fakeIDB").Call("failNextWrite", "QuotaExceededError")
			}
			done := make(chan error, 1)
			SetSecureBatch(map[string]string{generationKey: "next", rowKey: "replacement"}, map[string]string{generationKey: expected}, func(err error) { done <- err })
			err := waitResult(t, done)
			if err == nil {
				t.Fatal("failed commit acknowledged success")
			}
			if failure == "generation" && !errors.Is(err, atrest.ErrRecordChanged) {
				t.Fatalf("lost generation CAS identity: %v", err)
			}
			if GetString(generationKey) != "original" || GetString(rowKey) != "original row" {
				t.Fatal("failed commit advanced cache")
			}
			for key, want := range map[string]string{generationKey: "original", rowKey: "original row"} {
				value, present, err := GetSecure(key)
				if err != nil || !present || value != want {
					t.Fatalf("partial strict batch: %s=%q %t %v", key, value, present, err)
				}
			}
		})
	}
}

func TestIMV4SecureBulkReadRejectsInvalidRecordTypes(t *testing.T) {
	initFixture(t)
	const key = "cashflux:dataset:record:imv4:invalid"
	err := recordTransaction(true, func(tx js.Value, _ func(js.Func), _ func(error)) {
		tx.Call("objectStore", idbStoreName).Call("put", js.ValueOf(map[string]any{"unreadable": true}), key)
	})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	SetSecureBatch(map[string]string{key: "replacement"}, nil, func(err error) { done <- err })
	if err := waitResult(t, done); err == nil {
		t.Fatal("unreadable record was silently replaced")
	}
}

func TestIMV4PublishedBatchReloadUsesOneReadTransaction(t *testing.T) {
	initFixture(t)
	keys := []string{"cashflux:dataset:record:imv4:a", "cashflux:dataset:record:imv4:b"}
	for _, key := range keys {
		put(t, key, "durable row")
		SetMemory(key, "stale cache")
	}
	control := js.Global().Get("__fakeIDB")
	before := control.Call("transactionCount").Int()
	reloadPublishedBatch(keys)
	if count := control.Call("transactionCount").Int() - before; count != 1+len(keys) {
		t.Fatalf("batch reload used %d transactions, want one record read plus %d key-state checks", count, len(keys))
	}
	for _, key := range keys {
		if GetString(key) != "durable row" {
			t.Fatalf("stale batch cache for %s", key)
		}
	}
}

func TestIMV4BatchNotificationKeepsLegacyGenerationHint(t *testing.T) {
	previous := channel
	defer func() { channel = previous }()
	called := false
	post := js.FuncOf(func(_ js.Value, args []js.Value) any {
		called = true
		if got := args[0].Get("key").String(); got != "cashflux:dataset:gen" {
			t.Errorf("legacy invalidation=%s", got)
		}
		if args[0].Get("keys").Length() != 2 {
			t.Error("batch invalidation lost keys")
		}
		return nil
	})
	defer post.Release()
	channel = js.Global().Get("Object").New()
	channel.Set("postMessage", post)
	publishBatch([]string{"cashflux:dataset:record:imv4", "cashflux:dataset:gen"})
	if !called {
		t.Fatal("batch notification missing")
	}
}
