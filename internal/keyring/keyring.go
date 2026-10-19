// SPDX-License-Identifier: MIT

package keyring

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// FormatVersion is the current keyring blob format version.
const FormatVersion = 2

// KeySize is the size in bytes of a workspace data key and of every subkey.
const KeySize = 32

// KeyIDSize is the size in bytes of a workspace key identifier.
const KeyIDSize = 16

// NonceSize is the AES-GCM nonce size used by Seal and by key wrapping.
const NonceSize = 12

// PasscodeSaltSize is the size in bytes of the PBKDF2 salt stored in a
// passcode wrap entry.
const PasscodeSaltSize = 16

// PasscodeIterations is the PBKDF2-SHA-256 iteration count for passcode wrap
// entries (OWASP recommendation for PBKDF2-SHA-256).
const PasscodeIterations = 600_000

// PRFOutputSize is the required size in bytes of a WebAuthn PRF output used
// as passkey wrap input keying material.
const PRFOutputSize = 32

// HKDF info strings for the workspace subkeys.
const (
	// SubkeyData labels the subkey that seals workspace data (changes and
	// snapshots).
	SubkeyData = "cfv2/data"
	// SubkeyAttachments labels the subkey that seals attachment contents.
	SubkeyAttachments = "cfv2/attachments"
	// SubkeyAttachMAC labels the subkey that derives attachment content
	// addresses (keyed hashes).
	SubkeyAttachMAC = "cfv2/attach-mac"
	// SubkeyVault labels the subkey that seals stored credentials (the vault).
	SubkeyVault = "cfv2/vault"
	// SubkeyMetadata authenticates the complete canonical keyring metadata.
	SubkeyMetadata = "cfv2/keyring-metadata/v2"
	// SubkeyKeyCheck authenticates a WDK's key ID and version.
	SubkeyKeyCheck = "cfv2/key-check/v2"
)

// HKDF info strings for wrap key-encryption keys.
const (
	// infoRecovery labels the KEK derived from a recovery secret.
	infoRecovery = "cfv2/recovery"
	// infoPasskey labels the KEK derived from a passkey PRF output.
	infoPasskey = "cfv2/passkey"
)

// Sentinel errors returned by this package. Use errors.Is to test for them.
var (
	// ErrInvalidKeyring reports a keyring that fails validation.
	ErrInvalidKeyring = errors.New("keyring: invalid keyring")
	// ErrUnsupportedFormat reports a keyring blob whose FormatVersion this
	// build does not understand.
	ErrUnsupportedFormat = errors.New("keyring: unsupported format version")
	// ErrInvalidKey reports key material of the wrong size or a destroyed key.
	ErrInvalidKey = errors.New("keyring: invalid key")
	// ErrVersionNotFound reports a key version that is not in the keyring.
	ErrVersionNotFound = errors.New("keyring: key version not found")
	// ErrWrapNotFound reports a wrap entry that is not in the keyring.
	ErrWrapNotFound = errors.New("keyring: wrap entry not found")
	// ErrDuplicateWrap reports an attempt to add a second wrap entry with the
	// same kind and ID for one key version.
	ErrDuplicateWrap = errors.New("keyring: duplicate wrap entry")
	// ErrUnwrapFailed reports that a wrap entry could not be unwrapped with
	// the supplied secret.
	ErrUnwrapFailed = errors.New("keyring: unwrap failed")
	// ErrAuthenticationFailed reports a failed cryptographic authentication:
	// wrong key, context, confirmation code, MAC or ciphertext.
	ErrAuthenticationFailed = errors.New("keyring: authentication failed")
	// ErrInvalidRecoveryCode reports a recovery code that is malformed or
	// whose checksum does not match (most likely a typo).
	ErrInvalidRecoveryCode = errors.New("keyring: invalid recovery code")
	// ErrInvalidSecret reports an unwrap secret that does not match the wrap
	// kind being opened (for example a missing passcode or PRF output).
	ErrInvalidSecret = errors.New("keyring: invalid unwrap secret")
	// ErrRollback reports a keyring older than the caller's persisted version floor.
	ErrRollback = errors.New("keyring: rollback detected")
)

// AuthenticationError reports a failed cryptographic authentication, including
// AES-GCM, metadata/transfer MACs and pairing SAS confirmation. It matches
// ErrAuthenticationFailed with errors.Is.
type AuthenticationError struct {
	// Operation names what was being opened, for example "open" or "unwrap".
	Operation string
	// Err is the underlying cipher error, if any.
	Err error
}

// Error implements the error interface.
func (e *AuthenticationError) Error() string {
	if e == nil {
		return ErrAuthenticationFailed.Error()
	}
	if e.Err != nil {
		return fmt.Sprintf("keyring: %s: authentication failed: %v", e.Operation, e.Err)
	}
	return fmt.Sprintf("keyring: %s: authentication failed", e.Operation)
}

// Is reports whether target is ErrAuthenticationFailed.
func (e *AuthenticationError) Is(target error) bool {
	return target == ErrAuthenticationFailed
}

// Unwrap returns the underlying cipher error.
func (e *AuthenticationError) Unwrap() error { return e.Err }

// KeyID identifies one workspace data key. It is random, not secret, and is
// bound into wrap authenticated data so a wrapped key cannot be moved to a
// different key version silently.
type KeyID [KeyIDSize]byte

// NewKeyID returns a fresh random key ID.
func NewKeyID() (KeyID, error) {
	var id KeyID
	if _, err := rand.Read(id[:]); err != nil {
		return KeyID{}, fmt.Errorf("keyring: generate key id: %w", err)
	}
	return id, nil
}

// ParseKeyID converts raw bytes (exactly KeyIDSize long) into a KeyID.
func ParseKeyID(raw []byte) (KeyID, error) {
	if len(raw) != KeyIDSize {
		return KeyID{}, fmt.Errorf("%w: key id is %d bytes, want %d", ErrInvalidKey, len(raw), KeyIDSize)
	}
	var id KeyID
	copy(id[:], raw)
	return id, nil
}

// Bytes returns a copy of the raw key ID bytes.
func (id KeyID) Bytes() []byte {
	out := make([]byte, KeyIDSize)
	copy(out, id[:])
	return out
}

// String returns the key ID as lowercase hex, for logs and display.
func (id KeyID) String() string { return hex.EncodeToString(id[:]) }

// IsZero reports whether the key ID is all zeroes (unset).
func (id KeyID) IsZero() bool { return subtle.ConstantTimeCompare(id[:], make([]byte, KeyIDSize)) == 1 }

// MarshalJSON encodes the key ID as a base64 JSON string.
func (id KeyID) MarshalJSON() ([]byte, error) {
	return json.Marshal(base64.StdEncoding.EncodeToString(id[:]))
}

// UnmarshalJSON decodes a base64 JSON string into the key ID, rejecting any
// value that is not exactly KeyIDSize bytes.
func (id *KeyID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("%w: key id: %v", ErrInvalidKeyring, err)
	}
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return fmt.Errorf("%w: key id is not base64: %v", ErrInvalidKeyring, err)
	}
	parsed, err := ParseKeyID(raw)
	if err != nil {
		return err
	}
	*id = parsed
	return nil
}

// WrapKind identifies how a wrap entry protects the workspace data key.
type WrapKind string

// The wrap entry kinds stored in a keyring.
const (
	// WrapPasscode wraps the WDK under a key derived from the user's
	// passcode with PBKDF2-SHA-256.
	WrapPasscode WrapKind = "passcode"
	// WrapRecovery wraps the WDK under a key derived from the 128-bit
	// recovery secret.
	WrapRecovery WrapKind = "recovery"
	// WrapPasskey wraps the WDK under a key derived from a WebAuthn PRF
	// output.
	WrapPasskey WrapKind = "passkey"
	// WrapDevice wraps the WDK under a device key held by the platform,
	// reached through the Wrapper interface.
	WrapDevice WrapKind = "device"
)

// Valid reports whether k is a known wrap kind.
func (k WrapKind) Valid() bool {
	switch k {
	case WrapPasscode, WrapRecovery, WrapPasskey, WrapDevice:
		return true
	default:
		return false
	}
}

// WrapEntry is one wrapped copy of a workspace data key. Byte fields are
// encoded as base64 in the keyring blob. An entry contains everything needed
// to attempt an unwrap except the secret itself.
type WrapEntry struct {
	// Kind is how this entry is protected.
	Kind WrapKind `json:"kind"`
	// ID distinguishes entries of the same kind (a device ID, or a short
	// label). Empty means the single default entry of that kind. Passkey
	// entries are distinguished by CredentialID instead.
	ID string `json:"id,omitempty"`
	// Salt is the PBKDF2 salt (passcode entries only). It is generated when
	// the entry is created and stored here, so derivation runs once per
	// unlock, never per save.
	Salt []byte `json:"salt,omitempty"`
	// Iterations is the PBKDF2 iteration count (passcode entries only).
	Iterations int `json:"iterations,omitempty"`
	// CredentialID is the WebAuthn credential ID (passkey entries only).
	// It is not secret.
	CredentialID []byte `json:"credentialId,omitempty"`
	// PRFSalt is the salt fed to the WebAuthn PRF extension (passkey
	// entries only). It is not secret.
	PRFSalt []byte `json:"prfSalt,omitempty"`
	// Nonce is the AES-GCM nonce used to wrap the WDK. Device entries may
	// carry whatever nonce their Wrapper produced.
	Nonce []byte `json:"nonce"`
	// Wrapped is the AES-GCM ciphertext of the WDK (ciphertext plus tag).
	Wrapped []byte `json:"wrapped"`
}

// Clone returns a deep copy of the entry.
func (e WrapEntry) Clone() WrapEntry {
	out := e
	out.Salt = cloneBytes(e.Salt)
	out.CredentialID = cloneBytes(e.CredentialID)
	out.PRFSalt = cloneBytes(e.PRFSalt)
	out.Nonce = cloneBytes(e.Nonce)
	out.Wrapped = cloneBytes(e.Wrapped)
	return out
}

// identity returns the (kind, id) pair that distinguishes this entry within
// one key version. Passkey entries fall back to their credential ID when ID
// is empty.
func (e WrapEntry) identity() (WrapKind, string) {
	id := e.ID
	if id == "" && e.Kind == WrapPasskey && len(e.CredentialID) > 0 {
		id = base64.StdEncoding.EncodeToString(e.CredentialID)
	}
	return e.Kind, id
}

// VersionedKey is one key version: its identity plus every way it can be
// unwrapped. Old versions stay in the keyring so older ciphertext remains
// readable after a rotation.
type VersionedKey struct {
	// KeyID is the random identifier of this key version.
	KeyID KeyID `json:"keyId"`
	// Version is the key version number, starting at 1 and increasing by
	// one per rotation.
	Version uint32 `json:"version"`
	// KeyCheck authenticates this key's identity and material under a WDK subkey.
	KeyCheck []byte `json:"keyCheck"`
	// Wraps holds the wrap entries for this version. It may be empty
	// transiently (a freshly rotated key whose wraps are not yet added);
	// Parse rejects a keyring whose current version has no wraps.
	Wraps []WrapEntry `json:"wraps"`
}

// Clone returns a deep copy of the versioned key.
func (v VersionedKey) Clone() VersionedKey {
	out := VersionedKey{KeyID: v.KeyID, Version: v.Version, KeyCheck: cloneBytes(v.KeyCheck)}
	if v.Wraps != nil {
		out.Wraps = make([]WrapEntry, len(v.Wraps))
		for i, w := range v.Wraps {
			out.Wraps[i] = w.Clone()
		}
	}
	return out
}

// findWrap returns the entry with the given kind and ID, or nil.
func (v *VersionedKey) findWrap(kind WrapKind, id string) *WrapEntry {
	for i := range v.Wraps {
		k, wid := v.Wraps[i].identity()
		if k == kind && wid == id {
			return &v.Wraps[i]
		}
	}
	return nil
}

// Keyring is the persisted set of wrapped workspace keys for one workspace.
// It contains no plaintext key material and is safe to store on the server.
type Keyring struct {
	// FormatVersion is the blob format version (FormatVersion).
	FormatVersion int `json:"formatVersion"`
	// CurrentVersion is the key version used to seal new data.
	CurrentVersion uint32 `json:"currentVersion"`
	// Keys holds every retained key version, oldest first.
	Keys []VersionedKey `json:"keys"`
	// MetadataMAC authenticates all fields above, including every wrap byte,
	// under the current WDK. It is refreshed by Marshal, never trusted alone.
	MetadataMAC []byte `json:"metadataMAC"`
}

// Clone returns a deep copy of the keyring.
func (kr *Keyring) Clone() *Keyring {
	if kr == nil {
		return nil
	}
	out := &Keyring{FormatVersion: kr.FormatVersion, CurrentVersion: kr.CurrentVersion, MetadataMAC: cloneBytes(kr.MetadataMAC)}
	if kr.Keys != nil {
		out.Keys = make([]VersionedKey, len(kr.Keys))
		for i, k := range kr.Keys {
			out.Keys[i] = k.Clone()
		}
	}
	return out
}

// version returns the versioned key with the given version, or nil.
func (kr *Keyring) version(version uint32) *VersionedKey {
	if kr == nil {
		return nil
	}
	for i := range kr.Keys {
		if kr.Keys[i].Version == version {
			return &kr.Keys[i]
		}
	}
	return nil
}

// Current returns the current key version's entry set, or nil if the keyring
// is inconsistent. The returned value aliases the keyring; treat it as
// read-only.
func (kr *Keyring) Current() *VersionedKey {
	if kr == nil {
		return nil
	}
	return kr.version(kr.CurrentVersion)
}

// Versions returns the retained key version numbers in stored order.
func (kr *Keyring) Versions() []uint32 {
	if kr == nil {
		return nil
	}
	out := make([]uint32, len(kr.Keys))
	for i, k := range kr.Keys {
		out[i] = k.Version
	}
	return out
}

// WorkspaceKey is an unwrapped workspace data key held in memory while the
// workspace is unlocked. Call Destroy when locking. The zero value is not a
// usable key.
type WorkspaceKey struct {
	keyID    KeyID
	version  uint32
	material []byte
}

// NewWorkspaceKey generates a fresh random workspace data key at version 1.
func NewWorkspaceKey() (*WorkspaceKey, error) {
	id, err := NewKeyID()
	if err != nil {
		return nil, err
	}
	material := make([]byte, KeySize)
	if _, err := rand.Read(material); err != nil {
		return nil, fmt.Errorf("keyring: generate workspace key: %w", err)
	}
	return &WorkspaceKey{keyID: id, version: 1, material: material}, nil
}

// workspaceKeyFromMaterial builds a WorkspaceKey from unwrapped material,
// copying it.
func workspaceKeyFromMaterial(keyID KeyID, version uint32, material []byte) (*WorkspaceKey, error) {
	if len(material) != KeySize {
		return nil, fmt.Errorf("%w: workspace key is %d bytes, want %d", ErrInvalidKey, len(material), KeySize)
	}
	if version == 0 {
		return nil, fmt.Errorf("%w: workspace key version is zero", ErrInvalidKey)
	}
	return &WorkspaceKey{keyID: keyID, version: version, material: cloneBytes(material)}, nil
}

// KeyID returns the identifier of this key version.
func (wk *WorkspaceKey) KeyID() KeyID {
	if wk == nil {
		return KeyID{}
	}
	return wk.keyID
}

// Version returns the key version number.
func (wk *WorkspaceKey) Version() uint32 {
	if wk == nil {
		return 0
	}
	return wk.version
}

// Material returns a copy of the raw WDK bytes. Prefer the Subkey helpers;
// the raw material is needed only for device pairing transfer. Zero the copy
// when done. It returns nil for a destroyed key.
func (wk *WorkspaceKey) Material() []byte {
	if wk == nil || wk.material == nil {
		return nil
	}
	return cloneBytes(wk.material)
}

// Destroyed reports whether Destroy has been called.
func (wk *WorkspaceKey) Destroyed() bool { return wk == nil || wk.material == nil }

// Destroy zeroes the key material. The key must not be used afterwards.
func (wk *WorkspaceKey) Destroy() {
	if wk == nil {
		return
	}
	clearBytes(wk.material)
	wk.material = nil
}

// NewKeyring returns an empty keyring whose only (and current) version is
// wk's version, with no wrap entries yet. Add at least one wrap before
// persisting or discarding wk.
func NewKeyring(wk *WorkspaceKey) (*Keyring, error) {
	if wk.Destroyed() {
		return nil, fmt.Errorf("%w: cannot build a keyring from a destroyed key", ErrInvalidKey)
	}
	check, err := wk.keyCheck()
	if err != nil {
		return nil, err
	}
	return &Keyring{
		FormatVersion:  FormatVersion,
		CurrentVersion: wk.version,
		Keys: []VersionedKey{{
			KeyID:    wk.keyID,
			Version:  wk.version,
			KeyCheck: check,
			Wraps:    []WrapEntry{},
		}},
	}, nil
}

// cloneBytes returns a copy of b, preserving nil.
func cloneBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

// clearBytes zeroes b in place.
func clearBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// randomBytes returns n cryptographically random bytes.
func randomBytes(n int) ([]byte, error) {
	out := make([]byte, n)
	if _, err := rand.Read(out); err != nil {
		return nil, fmt.Errorf("keyring: read randomness: %w", err)
	}
	return out, nil
}
