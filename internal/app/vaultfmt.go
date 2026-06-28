// SPDX-License-Identifier: MIT

// Package app — pure vault-format helpers for the WebAuthn PRF passcode vault
// (C282). These functions are platform-independent so they can be unit-tested
// on native Go without a browser. The PRF vault stores the passcode encrypted
// under a key derived from the WebAuthn PRF output; this gives the passkey
// unlock path access to the passcode, which it hands to onAppUnlocked exactly
// as the passcode-input path does — the dataset decryption is unchanged.
package app

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

// prfVault is the JSON representation of an AES-GCM ciphertext produced by
// encryptVaultWithPRF. The vault stores the session passcode (encrypted) so
// the passkey unlock path can recover it and call the same onAppUnlocked
// function the passcode input path uses.
//
// Fields:
//   - V:      vault format version (always vaultVersion).
//   - IV:     base64-encoded 12-byte AES-GCM nonce.
//   - Cipher: base64-encoded ciphertext + 16-byte GCM authentication tag.
type prfVault struct {
	V      int    `json:"v"`
	IV     string `json:"iv"`
	Cipher string `json:"cipher"`
}

// vaultVersion is the current PRF vault format version. Increment on breaking
// changes to the format; parseVault rejects vaults with a mismatched version.
const vaultVersion = 1

// marshalVault encodes iv and cipher into the canonical vault JSON bytes.
// Both iv and cipher must be non-nil and non-empty.
func marshalVault(iv, cipher []byte) ([]byte, error) {
	if len(iv) == 0 {
		return nil, errors.New("webauthn vault: iv must be non-empty")
	}
	if len(cipher) == 0 {
		return nil, errors.New("webauthn vault: cipher must be non-empty")
	}
	v := prfVault{
		V:      vaultVersion,
		IV:     base64.StdEncoding.EncodeToString(iv),
		Cipher: base64.StdEncoding.EncodeToString(cipher),
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("webauthn vault: marshal: %w", err)
	}
	return data, nil
}

// parseVault decodes vault JSON bytes into the IV and cipher byte slices.
// Returns an error for any malformed, incomplete, or version-mismatched input.
// Callers must treat a parse error as a recoverable failure — never wipe data.
func parseVault(data []byte) (iv, cipher []byte, err error) {
	var v prfVault
	if jsonErr := json.Unmarshal(data, &v); jsonErr != nil {
		return nil, nil, fmt.Errorf("webauthn vault: parse: %w", jsonErr)
	}
	if v.V != vaultVersion {
		return nil, nil, fmt.Errorf("webauthn vault: unknown version %d (want %d)", v.V, vaultVersion)
	}
	if v.IV == "" {
		return nil, nil, errors.New("webauthn vault: missing iv field")
	}
	if v.Cipher == "" {
		return nil, nil, errors.New("webauthn vault: missing cipher field")
	}
	ivBytes, err := base64.StdEncoding.DecodeString(v.IV)
	if err != nil {
		return nil, nil, fmt.Errorf("webauthn vault: bad iv base64: %w", err)
	}
	cipherBytes, err := base64.StdEncoding.DecodeString(v.Cipher)
	if err != nil {
		return nil, nil, fmt.Errorf("webauthn vault: bad cipher base64: %w", err)
	}
	if len(ivBytes) == 0 {
		return nil, nil, errors.New("webauthn vault: iv decoded to empty")
	}
	if len(cipherBytes) == 0 {
		return nil, nil, errors.New("webauthn vault: cipher decoded to empty")
	}
	return ivBytes, cipherBytes, nil
}
