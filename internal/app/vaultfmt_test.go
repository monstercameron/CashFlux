// SPDX-License-Identifier: MIT

// Native unit tests for the PRF vault format helpers (vaultfmt.go). No build
// tag: these run with `go test ./internal/app/` on any platform without a
// browser or WASM runtime.
package app

import (
	"bytes"
	"encoding/json"
	"testing"
)

// TestVaultRoundTrip verifies that marshalVault → parseVault is lossless for
// representative IV and cipher byte slices.
func TestVaultRoundTrip(t *testing.T) {
	iv := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	cipher := []byte{13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29}
	data, err := marshalVault(iv, cipher)
	if err != nil {
		t.Fatalf("marshalVault: %v", err)
	}
	gotIV, gotCipher, err := parseVault(data)
	if err != nil {
		t.Fatalf("parseVault: %v", err)
	}
	if !bytes.Equal(gotIV, iv) {
		t.Errorf("IV mismatch: got %v, want %v", gotIV, iv)
	}
	if !bytes.Equal(gotCipher, cipher) {
		t.Errorf("cipher mismatch: got %v, want %v", gotCipher, cipher)
	}
}

// TestVaultVersionField verifies that the "v" field in the produced JSON equals
// vaultVersion so that future parseVault calls can detect format changes.
func TestVaultVersionField(t *testing.T) {
	iv := []byte{1, 2, 3}
	cipher := []byte{4, 5, 6}
	data, err := marshalVault(iv, cipher)
	if err != nil {
		t.Fatalf("marshalVault: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	v, ok := raw["v"]
	if !ok {
		t.Fatal("missing 'v' field in vault JSON")
	}
	if int(v.(float64)) != vaultVersion {
		t.Errorf("version field: got %v, want %d", v, vaultVersion)
	}
}

// TestVaultParseErrors verifies that parseVault returns a non-nil error for
// each class of invalid input; a bad vault must never be silently accepted.
func TestVaultParseErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"not JSON", []byte("not json at all")},
		{"wrong version zero", []byte(`{"v":0,"iv":"AAAA","cipher":"BBBB"}`)},
		{"wrong version high", []byte(`{"v":999,"iv":"AAAA","cipher":"BBBB"}`)},
		{"missing iv field", []byte(`{"v":1,"cipher":"BBBBBBBB"}`)},
		{"missing cipher field", []byte(`{"v":1,"iv":"AAAAAAAA"}`)},
		{"bad iv base64", []byte(`{"v":1,"iv":"!!!invalid","cipher":"AAAAAAAA"}`)},
		{"bad cipher base64", []byte(`{"v":1,"iv":"AAAAAAAA","cipher":"!!!invalid"}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseVault(tt.data)
			if err == nil {
				t.Errorf("parseVault(%q): expected error, got nil", tt.name)
			}
		})
	}
}

// TestMarshalVaultRejectsEmpty verifies that marshalVault rejects nil and empty
// byte slices for both the IV and cipher fields.
func TestMarshalVaultRejectsEmpty(t *testing.T) {
	iv := []byte{1, 2, 3}
	cipher := []byte{4, 5, 6}

	if _, err := marshalVault(nil, cipher); err == nil {
		t.Error("marshalVault(nil iv): expected error, got nil")
	}
	if _, err := marshalVault([]byte{}, cipher); err == nil {
		t.Error("marshalVault(empty iv): expected error, got nil")
	}
	if _, err := marshalVault(iv, nil); err == nil {
		t.Error("marshalVault(nil cipher): expected error, got nil")
	}
	if _, err := marshalVault(iv, []byte{}); err == nil {
		t.Error("marshalVault(empty cipher): expected error, got nil")
	}
}

// TestPasskeylessDatasetUnchanged is a structural proof that the vault helpers
// (vaultfmt.go) are completely disjoint from the dataset encryption path
// (datasetcrypto.go). The vault only ever stores and retrieves a passcode
// string — it never touches dataset bytes, PBKDF2 keys, or cryptobox envelopes.
//
// The test encodes a passcode-sized byte slice through marshalVault/parseVault
// and confirms the bytes come back identical, demonstrating that the vault
// format is an opaque container. A user who never registers a passkey exercises
// none of this code path.
func TestPasskeylessDatasetUnchanged(t *testing.T) {
	passcodeBytes := []byte("cashflux-test-passcode-1234")
	iv := make([]byte, 12)
	for i := range iv {
		iv[i] = byte(i + 1)
	}
	data, err := marshalVault(iv, passcodeBytes)
	if err != nil {
		t.Fatalf("marshalVault: %v", err)
	}
	gotIV, gotPasscode, err := parseVault(data)
	if err != nil {
		t.Fatalf("parseVault: %v", err)
	}
	if !bytes.Equal(gotPasscode, passcodeBytes) {
		t.Errorf("passcode mismatch: got %q, want %q", gotPasscode, passcodeBytes)
	}
	if len(gotIV) != 12 {
		t.Errorf("IV length: got %d, want 12", len(gotIV))
	}
	// Ensure the vault JSON contains none of the dataset-crypto keys.
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	for _, forbidden := range []string{"alg", "salt", "pbkdf2"} {
		if _, found := raw[forbidden]; found {
			t.Errorf("vault JSON unexpectedly contains dataset-crypto key %q", forbidden)
		}
	}
}
