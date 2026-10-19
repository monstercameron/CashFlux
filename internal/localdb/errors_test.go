// SPDX-License-Identifier: MIT
package localdb

import (
	"errors"
	"testing"
)

// TestClassifyStorageError pins the DOMException-name mapping: only
// QuotaExceededError may become a QuotaError, and abort / security /
// unsupported failures stay distinguishable by sentinel and by name.
func TestClassifyStorageError(t *testing.T) {
	cases := []struct {
		name      string
		wantQuota bool
		wantIs    error // sentinel that must match, or nil
		wantNotIs error // sentinel that must NOT match, or nil
	}{
		{name: "QuotaExceededError", wantQuota: true, wantNotIs: ErrStorageAborted},
		{name: "AbortError", wantIs: ErrStorageAborted, wantNotIs: ErrStorageSecurity},
		{name: "SecurityError", wantIs: ErrStorageSecurity, wantNotIs: ErrStorageUnsupported},
		{name: "NotSupportedError", wantIs: ErrStorageUnsupported, wantNotIs: ErrStorageAborted},
		{name: "ConstraintError", wantNotIs: ErrStorageAborted},
		{name: "VersionError"},
		{name: "UnknownError"},
		{name: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := classifyStorageError(tc.name, "browser said no")
			if err == nil {
				t.Fatal("classifyStorageError returned nil")
			}
			var quota *QuotaError
			if got := errors.As(err, &quota); got != tc.wantQuota {
				t.Errorf("errors.As(*QuotaError) = %v, want %v (err=%v)", got, tc.wantQuota, err)
			}
			if !tc.wantQuota {
				var storage *StorageError
				if !errors.As(err, &storage) {
					t.Fatalf("expected *StorageError, got %T (%v)", err, err)
				}
				if storage.Name != tc.name {
					t.Errorf("StorageError.Name = %q, want %q", storage.Name, tc.name)
				}
			}
			if tc.wantIs != nil && !errors.Is(err, tc.wantIs) {
				t.Errorf("errors.Is(err, %v) = false for %v", tc.wantIs, err)
			}
			if tc.wantNotIs != nil && errors.Is(err, tc.wantNotIs) {
				t.Errorf("errors.Is(err, %v) = true for %v", tc.wantNotIs, err)
			}
			if got := storageErrorName(err); got != tc.name {
				t.Errorf("storageErrorName = %q, want %q", got, tc.name)
			}
		})
	}
}

func TestClassifyStorageErrorMessages(t *testing.T) {
	if got := classifyStorageError("AbortError", "").Error(); got == "" {
		t.Fatal("empty message should still produce a readable error")
	}
	if got := classifyStorageError("", "").Error(); got == "" {
		t.Fatal("empty name and message should still produce a readable error")
	}
	var quota *QuotaError
	err := classifyStorageError("QuotaExceededError", "disk full")
	if !errors.As(err, &quota) || quota.Name != "QuotaExceededError" {
		t.Fatalf("quota error = %#v", err)
	}
}
