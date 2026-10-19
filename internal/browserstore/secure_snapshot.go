// SPDX-License-Identifier: MIT

//go:build js && wasm

package browserstore

import (
	"fmt"
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/atrest"
)

// SecureSnapshot holds independently authenticated values from one IndexedDB
// read transaction. Errors retain unreadable records rather than hiding them as
// absent. It is detached from the mutable cross-tab cache.
type SecureSnapshot struct {
	// Values contains only values whose authentication succeeded.
	Values map[string]string
	// Errors retains the failure for each unreadable selected record.
	Errors  map[string]error
	records map[string]string
}

// SetSecureSnapshotBatch commits values only if the selected records still have
// the snapshot's exact ciphertexts (or remain absent). This also permits safe
// recovery from an unreadable value without weakening its compare-and-swap.
func SetSecureSnapshotBatch(values map[string]string, snapshot SecureSnapshot, conditions []string, done func(error)) {
	compared := make(map[string]*string, len(conditions))
	for _, key := range conditions {
		if raw, present := snapshot.records[key]; present {
			copy := raw
			compared[key] = &copy
		} else {
			compared[key] = nil
		}
	}
	setSecureBatch(values, nil, compared, done)
}

// Read returns the snapshot's value, presence, and authentication error.
func (s SecureSnapshot) Read(key string) (string, bool, error) {
	value, present := s.Values[key]
	if err := s.Errors[key]; err != nil {
		return "", true, err
	}
	return value, present, nil
}

// EncodedRecord returns the original stored bytes for an opaque recovery copy.
// It never decrypts an unreadable record or changes the live source.
func (s SecureSnapshot) EncodedRecord(key string) (string, bool) {
	value, present := s.records[key]
	return value, present
}

// ReadSecureSnapshot reads keys and prefix families together, including keys
// absent from this tab's cache. It blocks; call it from a Go goroutine. No data
// from different commits can be combined, even if decryption yields to a writer.
func ReadSecureSnapshot(keys, prefixes []string) (SecureSnapshot, error) {
	snapshot := SecureSnapshot{Values: map[string]string{}, Errors: map[string]error{}, records: map[string]string{}}
	if secureInitErr != nil {
		return snapshot, secureInitErr
	}
	mu.Lock()
	locked := secureLocked
	mu.Unlock()
	if locked {
		return snapshot, atrest.ErrLocked
	}
	wanted := map[string]bool{}
	for _, key := range keys {
		wanted[key] = true
	}
	stored := map[string]string{}
	err := recordTransaction(false, func(tx js.Value, retain func(js.Func), fail func(error)) {
		os := tx.Call("objectStore", idbStoreName)
		req := os.Call("getAllKeys")
		fn := js.FuncOf(func(js.Value, []js.Value) any {
			all := req.Get("result")
			for i := 0; i < all.Length(); i++ {
				key := all.Index(i).String()
				selected := wanted[key]
				for _, prefix := range prefixes {
					selected = selected || strings.HasPrefix(key, prefix)
				}
				if !selected || atrest.Classify(key) == atrest.ShouldNotPersist {
					continue
				}
				get := os.Call("get", key)
				read := js.FuncOf(func(js.Value, []js.Value) any {
					value := get.Get("result")
					if value.Type() == js.TypeString {
						stored[key] = value.String()
					} else {
						fail(fmt.Errorf("protected record %q has an invalid stored type", key))
						tx.Call("abort")
					}
					return nil
				})
				retain(read)
				get.Set("onsuccess", read)
			}
			return nil
		})
		retain(fn)
		req.Set("onsuccess", fn)
	})
	if err != nil {
		return snapshot, err
	}
	snapshot.records = stored
	for key, raw := range stored {
		value, err := storage().Decode(key, raw)
		if err != nil {
			snapshot.Errors[key] = &RecordError{Key: key, Err: err}
		} else {
			snapshot.Values[key] = value
		}
	}
	mu.Lock()
	locked = secureLocked
	mu.Unlock()
	if locked {
		return SecureSnapshot{}, atrest.ErrLocked
	}
	return snapshot, nil
}
