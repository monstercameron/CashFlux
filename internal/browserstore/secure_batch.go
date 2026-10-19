//go:build js && wasm

package browserstore

import (
	"context"
	"fmt"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/atrest"
)

// readSecureBatch observes all prior ciphertexts in one readonly transaction.
// The writer still compares those exact bytes inside its strict transaction:
// reading together is an optimization, never a substitute for generation CAS.
func readSecureBatch(keys []string) (map[string]string, error) {
	return readSecureBatchContext(context.Background(), keys)
}

func readSecureBatchContext(ctx context.Context, keys []string) (map[string]string, error) {
	values := make(map[string]string, len(keys))
	err := recordTransactionContext(ctx, false, func(tx js.Value, retain func(js.Func), fail func(error)) {
		readRecordRequests(tx, keys, retain, fail, func(key string, value js.Value) bool {
			if value.Type() == js.TypeString {
				values[key] = value.String()
			} else if !value.IsUndefined() && !value.IsNull() {
				fail(fmt.Errorf("protected record %q has an invalid stored type", key))
				tx.Call("abort")
				return false
			}
			return true
		})
	})
	return values, err
}

// publishBatch sends one key-only invalidation after the entire cache update.
// Values and credentials never travel over BroadcastChannel.
func publishBatch(keys []string) {
	if len(keys) == 0 || !channel.Truthy() {
		return
	}
	defer func() { _ = recover() }()
	if len(keys) == 1 {
		publish(keys[0]) // existing pages understand the single-record wire shape
		return
	}
	items := make([]any, len(keys))
	for i, key := range keys {
		items[i] = key
	}
	legacyKey := keys[0]
	for _, key := range keys {
		if key == "cashflux:dataset:gen" {
			legacyKey = key // older pages must still learn that their dataset is stale
			break
		}
	}
	channel.Call("postMessage", map[string]any{"keys": items, "key": legacyKey})
}

func reloadPublishedBatch(keys []string) {
	keys = reloadDeferredNames(keys)
	stored, readErr := readSecureBatch(keys)
	values := make(map[string]string, len(keys))
	failures := make(map[string]error, len(keys))
	sliceStart := time.Now()
	for _, key := range keys {
		if atrest.Classify(key) == atrest.ShouldNotPersist {
			continue
		}
		if readErr != nil {
			failures[key] = readErr
			continue
		}
		if raw, present := stored[key]; present {
			value, err := storage().Decode(key, raw)
			if err != nil {
				failures[key] = err
			} else {
				values[key] = value
			}
		}
		if time.Since(sliceStart) >= 12*time.Millisecond {
			time.Sleep(time.Millisecond)
			sliceStart = time.Now()
		}
	}
	// Watchers see the entire committed batch, never a partially refreshed cache.
	mu.Lock()
	for _, key := range keys {
		if atrest.Classify(key) == atrest.ShouldNotPersist {
			continue
		}
		delete(cache, key)
		delete(readErrors, key)
		if err := failures[key]; err != nil {
			readErrors[key] = err
		} else if value, present := values[key]; present {
			cache[key] = value
		}
	}
	mu.Unlock()
}
