// SPDX-License-Identifier: MIT

package keyring

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"fmt"
)

// LocalServerID is the ServerID used for workspaces that have never been
// associated with a sync server.
const LocalServerID = "local"

// Scope names the server and account a blob belongs to. Every ciphertext
// binds its scope in its authenticated data, so data written under one
// account or server cannot be opened — and therefore cannot merge — under
// another.
type Scope struct {
	// ServerID identifies the sync server, or LocalServerID ("local") for
	// local-only workspaces.
	ServerID string
	// AccountID identifies the account on that server. For local-only
	// workspaces it may be a locally generated ID or empty; it still
	// participates in the authenticated data.
	AccountID string
}

// LocalScope returns the local-only scope for the given account ID.
func LocalScope(accountID string) Scope {
	return Scope{ServerID: LocalServerID, AccountID: accountID}
}

// IsLocal reports whether the scope is local-only.
func (s Scope) IsLocal() bool { return s.ServerID == LocalServerID }

// aadDomain labels every value built by AAD.
const aadDomain = "cfv2/aad/v1"

// AAD builds the authenticated data for a sealed change or snapshot. The
// encoding is canonical and length-prefixed —
//
//	"cfv2/aad/v1" ‖ lp(ServerID) ‖ lp(AccountID) ‖ lp(workspaceID) ‖
//	lp(changeID) ‖ lp(keyVersion as 4-byte big-endian)
//
// where lp(x) is a 4-byte big-endian length followed by x — so no two
// different (scope, workspace, change, version) tuples can produce the same
// bytes by shifting characters between fields.
func AAD(scope Scope, workspaceID, changeID string, keyVersion uint32) []byte {
	var versionBytes [4]byte
	binary.BigEndian.PutUint32(versionBytes[:], keyVersion)
	return joinLengthPrefixed(
		[]byte(aadDomain),
		[]byte(scope.ServerID),
		[]byte(scope.AccountID),
		[]byte(workspaceID),
		[]byte(changeID),
		versionBytes[:],
	)
}

// wrapAAD builds the authenticated data binding a wrap entry to its kind,
// key ID, key version, and entry identity:
//
//	"cfv2/wrap/<kind>" ‖ lp(KeyID) ‖ lp(Version) ‖ lp(entryID)
//
// A wrapped key copied to a different version, kind, or entry therefore
// fails to unwrap instead of unlocking the wrong key.
func wrapAAD(kind WrapKind, keyID KeyID, version uint32, entryID string) []byte {
	var versionBytes [4]byte
	binary.BigEndian.PutUint32(versionBytes[:], version)
	return joinLengthPrefixed(
		[]byte("cfv2/wrap/"+string(kind)),
		keyID[:],
		versionBytes[:],
		[]byte(entryID),
	)
}

// joinLengthPrefixed concatenates parts, each preceded by its 4-byte
// big-endian length.
func joinLengthPrefixed(parts ...[]byte) []byte {
	total := 0
	for _, p := range parts {
		total += 4 + len(p)
	}
	out := make([]byte, 0, total)
	var lenBuf [4]byte
	for _, p := range parts {
		binary.BigEndian.PutUint32(lenBuf[:], uint32(len(p)))
		out = append(out, lenBuf[:]...)
		out = append(out, p...)
	}
	return out
}

// newGCM returns an AES-256-GCM AEAD for a 32-byte key.
func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("%w: key is %d bytes, want %d", ErrInvalidKey, len(key), KeySize)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("keyring: create AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("keyring: create GCM: %w", err)
	}
	return gcm, nil
}

// Seal encrypts plaintext with AES-256-GCM under key (a 32-byte subkey),
// binding aad as authenticated data. The result is nonce‖ciphertext with a
// fresh random 96-bit nonce, so sealing the same plaintext twice yields
// different bytes. aad should come from AAD so the blob names its scope,
// workspace, change, and key version.
func Seal(key, aad, plaintext []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce, err := randomBytes(NonceSize)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, NonceSize+len(plaintext)+gcm.Overhead())
	out = append(out, nonce...)
	out = gcm.Seal(out, nonce, plaintext, aad)
	return out, nil
}

// Open decrypts a value produced by Seal. It returns an *AuthenticationError
// (matching ErrAuthenticationFailed) when the key, the authenticated data,
// or the ciphertext do not match — callers must treat that as "this blob is
// not for us", never as a reason to discard or overwrite the blob.
func Open(key, aad, sealed []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(sealed) < NonceSize+gcm.Overhead() {
		return nil, &AuthenticationError{Operation: "open", Err: fmt.Errorf("sealed value is %d bytes, too short", len(sealed))}
	}
	nonce, ciphertext := sealed[:NonceSize], sealed[NonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, &AuthenticationError{Operation: "open", Err: err}
	}
	return plaintext, nil
}

// sealWithNonce is Seal with a caller-supplied nonce, split out so wrap
// entries can store nonce and ciphertext in separate fields. It returns the
// ciphertext (including the GCM tag) only.
func sealWithNonce(key, nonce, aad, plaintext []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(nonce) != NonceSize {
		return nil, fmt.Errorf("%w: nonce is %d bytes, want %d", ErrInvalidKey, len(nonce), NonceSize)
	}
	return gcm.Seal(nil, nonce, plaintext, aad), nil
}

// openWithNonce reverses sealWithNonce.
func openWithNonce(key, nonce, aad, ciphertext []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, &AuthenticationError{Operation: "unwrap", Err: fmt.Errorf("nonce is %d bytes, want %d", len(nonce), gcm.NonceSize())}
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, &AuthenticationError{Operation: "unwrap", Err: err}
	}
	return plaintext, nil
}
