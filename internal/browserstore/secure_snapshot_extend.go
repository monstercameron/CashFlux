//go:build js && wasm

package browserstore

import (
	"context"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/atrest"
)

// ExtendSecureSnapshotContext loads deferred repair inputs without re-reading
// an immutable journal. Every unchanged guard must have the exact same stored
// ciphertext (or absence) in both reads. The caller must still CAS all original
// and added repair inputs together before publishing a repaired dataset.
func ExtendSecureSnapshotContext(ctx context.Context, original SecureSnapshot, keys, unchanged []string) (SecureSnapshot, error) {
	selected := append(append([]string(nil), keys...), unchanged...)
	extra, err := ReadSecureSnapshotContext(ctx, selected, nil)
	if err != nil {
		return SecureSnapshot{}, err
	}
	for _, key := range unchanged {
		before, haveBefore := original.EncodedRecord(key)
		after, haveAfter := extra.EncodedRecord(key)
		if haveBefore != haveAfter || before != after {
			return SecureSnapshot{}, fmt.Errorf("snapshot guard %q changed while loading repair input: %w", key, atrest.ErrRecordChanged)
		}
	}
	result := SecureSnapshot{Values: map[string]string{}, Errors: map[string]error{}, records: map[string]string{}}
	for key, value := range original.Values {
		result.Values[key] = value
	}
	for key, value := range original.Errors {
		result.Errors[key] = value
	}
	for key, value := range original.records {
		result.records[key] = value
	}
	for key, value := range extra.records {
		result.records[key] = value
		delete(result.Values, key)
		delete(result.Errors, key)
	}
	for key, value := range extra.Values {
		result.Values[key] = value
	}
	for key, value := range extra.Errors {
		result.Errors[key] = value
	}
	return result, nil
}
