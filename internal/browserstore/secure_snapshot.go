// SPDX-License-Identifier: MIT

//go:build js && wasm

package browserstore

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"syscall/js"
	"time"

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
	SetSecureSnapshotBatchContext(context.Background(), values, snapshot, conditions, done)
}

// SetSecureSnapshotBatchContext cancels a queued repair before it can write when
// its caller's deadline expires. Success still requires the strict commit.
func SetSecureSnapshotBatchContext(ctx context.Context, values map[string]string, snapshot SecureSnapshot, conditions []string, done func(error)) {
	compared := make(map[string]*string, len(conditions))
	for _, key := range conditions {
		if raw, present := snapshot.records[key]; present {
			copy := raw
			compared[key] = &copy
		} else {
			compared[key] = nil
		}
	}
	setSecureBatchContext(ctx, values, nil, compared, done)
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
	return ReadSecureSnapshotContext(context.Background(), keys, prefixes)
}

// ReadSecureSnapshotContext bounds the entire read, including key-service
// authentication. An expired read never publishes a partial snapshot.
func ReadSecureSnapshotContext(ctx context.Context, keys, prefixes []string) (SecureSnapshot, error) {
	keys, prefixes = append([]string(nil), keys...), append([]string(nil), prefixes...)
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	type result struct {
		snapshot SecureSnapshot
		err      error
	}
	done := make(chan result, 1)
	go func() {
		s, err := readSecureSnapshot(ctx, keys, prefixes)
		if err == nil {
			migrated := false
			for key, readErr := range s.Errors {
				if !recoveryValueKey(key) || !errors.Is(readErr, atrest.ErrPlaintextRecord) {
					continue
				}
				if ctx.Err() != nil {
					err = ctx.Err()
					break
				}
				// Cold legacy values were deliberately not opened at boot.
				// This is an explicit recovery read: migrate with ciphertext
				// verification, then re-read the complete coherent selection.
				if migrationErr := openDeferredRecord(key); migrationErr != nil {
					err = migrationErr
					break
				}
				migrated = true
			}
			if migrated && err == nil {
				s, err = readSecureSnapshot(ctx, keys, prefixes)
			}
		}
		done <- result{s, err}
	}()
	select {
	case r := <-done:
		if err := ctx.Err(); err != nil {
			return SecureSnapshot{}, err
		}
		return r.snapshot, r.err
	case <-ctx.Done():
		return SecureSnapshot{}, fmt.Errorf("read authenticated snapshot: %w", ctx.Err())
	}
}

func readSecureSnapshot(ctx context.Context, keys, prefixes []string) (SecureSnapshot, error) {
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
	slog.Info("dataset hydration stage", "stage", "snapshot read", "status", "started")
	err := recordTransactionContext(ctx, false, func(tx js.Value, retain func(js.Func), fail func(error)) {
		os := tx.Call("objectStore", idbStoreName)
		req := os.Call("getAllKeys")
		fn := js.FuncOf(func(js.Value, []js.Value) any {
			all := req.Get("result")
			var selectedKeys []string
			for i := 0; i < all.Length(); i++ {
				key := all.Index(i).String()
				selected := wanted[key]
				for _, prefix := range prefixes {
					selected = selected || strings.HasPrefix(key, prefix)
				}
				if selected && atrest.Classify(key) != atrest.ShouldNotPersist {
					selectedKeys = append(selectedKeys, key)
				}
			}
			readRecordRequests(tx, selectedKeys, retain, fail, func(key string, value js.Value) bool {
				if value.Type() == js.TypeString {
					stored[key] = value.String()
				} else {
					fail(fmt.Errorf("protected record %q has an invalid stored type", key))
					tx.Call("abort")
					return false
				}
				return true
			})
			return nil
		})
		retain(fn)
		req.Set("onsuccess", fn)
	})
	if err != nil {
		return snapshot, err
	}
	slog.Info("dataset hydration stage", "stage", "snapshot read", "status", "complete", "records", len(stored))
	slog.Info("dataset hydration stage", "stage", "verify", "status", "started")
	snapshot.records = stored
	verifier := newEmptyRecordVerifier()
	for key, raw := range stored {
		if err := ctx.Err(); err != nil {
			return SecureSnapshot{}, err
		}
		value, err := verifier.decode(key, raw)
		if err != nil {
			snapshot.Errors[key] = &RecordError{Key: key, Err: err}
		} else {
			snapshot.Values[key] = value
		}
	}
	if err := verifier.finish(); err != nil {
		return SecureSnapshot{}, err
	}
	mu.Lock()
	locked = secureLocked
	mu.Unlock()
	if locked {
		return SecureSnapshot{}, atrest.ErrLocked
	}
	slog.Info("dataset hydration stage", "stage", "verify", "status", "complete", "records", len(stored), "unreadable", len(snapshot.Errors))
	return snapshot, nil
}
