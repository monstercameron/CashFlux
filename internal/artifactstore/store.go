// SPDX-License-Identifier: MIT

// Package artifactstore defines the interface for storing binary artifact blobs
// (uploaded images and other binary data) separately from the main dataset JSON.
// The concrete implementation uses IndexedDB in the browser; this pure package
// is platform-independent and unit-tests on native Go.
package artifactstore

import "errors"

// RecommendedQuota is the soft threshold (50 MB) below which blob storage is
// considered healthy. IndexedDB quotas vary by browser/origin, but 50 MB is a
// conservative amount that keeps usage well within typical browser grants.
const RecommendedQuota int64 = 50 << 20 // 50 MiB

// WarnThreshold is the fraction of RecommendedQuota at which a quota warning is
// surfaced to the user (90 %).
const WarnThreshold = 0.90

// Store is the interface for reading and writing binary artifact blobs. All
// methods are synchronous from the caller's perspective; the wasm implementation
// blocks on async IndexedDB callbacks via channels.
type Store interface {
	// Put stores the blob for the given artifact id, replacing any prior value.
	Put(id string, mime string, data []byte) error
	// Get retrieves the blob for the given artifact id. ok is false when the id
	// is not found; err is non-nil only on storage failures.
	Get(id string) (mime string, data []byte, ok bool, err error)
	// Delete removes the blob for the given artifact id. It is not an error if
	// the id was not present.
	Delete(id string) error
	// Usage returns a best-effort estimate of bytes used in the blob store.
	// Implementations that cannot query storage may return 0, nil.
	Usage() (bytes int64, err error)
}

// ErrUnavailable is returned when the underlying storage mechanism (e.g.
// IndexedDB) is not available in the current environment.
var ErrUnavailable = errors.New("artifactstore: storage unavailable")

// OverQuota reports whether usedBytes has reached or exceeded the threshold
// fraction of quotaBytes at which a warning should be shown. It returns false
// when quotaBytes is zero (unknown quota).
func OverQuota(usedBytes, quotaBytes int64) bool {
	if quotaBytes <= 0 {
		return false
	}
	return float64(usedBytes) >= float64(quotaBytes)*WarnThreshold
}

// NearLimit reports whether usedBytes is approaching or has exceeded
// RecommendedQuota, using the same WarnThreshold fraction.
func NearLimit(usedBytes int64) bool {
	return OverQuota(usedBytes, RecommendedQuota)
}
