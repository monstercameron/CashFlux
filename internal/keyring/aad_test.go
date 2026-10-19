// SPDX-License-Identifier: MIT

package keyring

import (
	"bytes"
	"errors"
	"testing"
)

func TestSealOpenRoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0x11}, KeySize)
	aad := AAD(Scope{ServerID: "srv-1", AccountID: "acct-1"}, "ws-1", "chg-1", 3)

	for _, plain := range [][]byte{nil, {}, []byte("a"), []byte("a longer change payload, sealed"), bytes.Repeat([]byte("x"), 4096)} {
		sealed, err := Seal(key, aad, plain)
		if err != nil {
			t.Fatalf("Seal(%d bytes): %v", len(plain), err)
		}
		if len(sealed) != NonceSize+len(plain)+16 {
			t.Fatalf("sealed size = %d, want nonce+plain+tag = %d", len(sealed), NonceSize+len(plain)+16)
		}
		opened, err := Open(key, aad, sealed)
		if err != nil {
			t.Fatalf("Open(%d bytes): %v", len(plain), err)
		}
		if !bytes.Equal(opened, plain) {
			t.Errorf("round trip mismatch for %d-byte plaintext", len(plain))
		}
	}

	// Sealing is randomised: same input, different bytes.
	a, _ := Seal(key, aad, []byte("same"))
	b, _ := Seal(key, aad, []byte("same"))
	if bytes.Equal(a, b) {
		t.Error("two Seals of the same plaintext produced identical bytes")
	}
}

func TestOpenRejectsEveryAADComponent(t *testing.T) {
	key := bytes.Repeat([]byte{0x22}, KeySize)
	base := Scope{ServerID: "srv-1", AccountID: "acct-1"}
	sealed, err := Seal(key, AAD(base, "ws-1", "chg-1", 2), []byte("payload"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	tests := []struct {
		name string
		aad  []byte
	}{
		{"different server", AAD(Scope{ServerID: "srv-2", AccountID: "acct-1"}, "ws-1", "chg-1", 2)},
		{"local instead of server", AAD(LocalScope("acct-1"), "ws-1", "chg-1", 2)},
		{"different account", AAD(Scope{ServerID: "srv-1", AccountID: "acct-2"}, "ws-1", "chg-1", 2)},
		{"empty account", AAD(Scope{ServerID: "srv-1"}, "ws-1", "chg-1", 2)},
		{"different workspace", AAD(base, "ws-2", "chg-1", 2)},
		{"different change", AAD(base, "ws-1", "chg-2", 2)},
		{"empty change (snapshot marker differs)", AAD(base, "ws-1", "", 2)},
		{"different key version", AAD(base, "ws-1", "chg-1", 3)},
		{"zero key version", AAD(base, "ws-1", "chg-1", 0)},
		{"nil aad", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Open(key, tc.aad, sealed)
			var authErr *AuthenticationError
			if !errors.As(err, &authErr) || !errors.Is(err, ErrAuthenticationFailed) {
				t.Fatalf("Open err = %v, want *AuthenticationError", err)
			}
		})
	}

	// Wrong key and corrupted ciphertext also fail authentication.
	if _, err := Open(bytes.Repeat([]byte{0x33}, KeySize), AAD(base, "ws-1", "chg-1", 2), sealed); !errors.Is(err, ErrAuthenticationFailed) {
		t.Errorf("wrong key err = %v", err)
	}
	corrupt := cloneBytes(sealed)
	corrupt[len(corrupt)-1] ^= 0xFF
	if _, err := Open(key, AAD(base, "ws-1", "chg-1", 2), corrupt); !errors.Is(err, ErrAuthenticationFailed) {
		t.Errorf("corrupt ciphertext err = %v", err)
	}
	for _, short := range [][]byte{nil, sealed[:NonceSize], sealed[:NonceSize+4]} {
		if _, err := Open(key, AAD(base, "ws-1", "chg-1", 2), short); !errors.Is(err, ErrAuthenticationFailed) {
			t.Errorf("truncated sealed (%d bytes) err = %v", len(short), err)
		}
	}
}

func TestAADHasNoAmbiguousConcatenation(t *testing.T) {
	// Without length prefixes, ("ab","c") and ("a","bc") would collide.
	a := AAD(Scope{ServerID: "ab", AccountID: "c"}, "ws", "chg", 1)
	b := AAD(Scope{ServerID: "a", AccountID: "bc"}, "ws", "chg", 1)
	if bytes.Equal(a, b) {
		t.Error("AAD collides under field-boundary shift (server/account)")
	}
	c := AAD(Scope{ServerID: "srv", AccountID: "acct"}, "wsx", "chg", 1)
	d := AAD(Scope{ServerID: "srv", AccountID: "acct"}, "ws", "xchg", 1)
	if bytes.Equal(c, d) {
		t.Error("AAD collides under field-boundary shift (workspace/change)")
	}
	// Deterministic for identical inputs.
	if !bytes.Equal(a, AAD(Scope{ServerID: "ab", AccountID: "c"}, "ws", "chg", 1)) {
		t.Error("AAD is not deterministic")
	}
}

func TestRecordAADBindsRecordTupleAndHasSeparateDomain(t *testing.T) {
	scope := Scope{ServerID: "server", AccountID: "account"}
	base := RecordAAD(scope, "workspace", "record", 2)
	if bytes.Equal(base, AAD(scope, "workspace", "record", 2)) {
		t.Fatal("record AAD shares the change AAD domain")
	}
	for name, got := range map[string][]byte{
		"server":    RecordAAD(Scope{ServerID: "other", AccountID: "account"}, "workspace", "record", 2),
		"account":   RecordAAD(Scope{ServerID: "server", AccountID: "other"}, "workspace", "record", 2),
		"workspace": RecordAAD(scope, "other", "record", 2),
		"record":    RecordAAD(scope, "workspace", "other", 2),
		"version":   RecordAAD(scope, "workspace", "record", 3),
	} {
		if bytes.Equal(base, got) {
			t.Errorf("RecordAAD did not bind %s", name)
		}
	}
}

func TestSealRejectsBadKeySizes(t *testing.T) {
	for _, size := range []int{0, 16, 24, 31, 33} {
		if _, err := Seal(make([]byte, size), nil, []byte("x")); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Seal with %d-byte key err = %v, want ErrInvalidKey", size, err)
		}
		if _, err := Open(make([]byte, size), nil, make([]byte, 64)); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Open with %d-byte key err = %v, want ErrInvalidKey", size, err)
		}
	}
}

func TestLocalScope(t *testing.T) {
	s := LocalScope("acct")
	if s.ServerID != "local" || !s.IsLocal() {
		t.Errorf("LocalScope = %+v", s)
	}
	if (Scope{ServerID: "srv"}).IsLocal() {
		t.Error("server scope reports local")
	}
}
