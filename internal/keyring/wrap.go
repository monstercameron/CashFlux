// SPDX-License-Identifier: MIT

package keyring

import (
	"crypto/pbkdf2"
	"crypto/sha256"
	"errors"
	"fmt"
)

// PBKDF2Deriver derives a key-encryption key from a passcode. It exists so
// the browser build can substitute WebCrypto (600,000 PBKDF2 iterations are
// slow in pure-Go wasm) and so tests can substitute a fast fake. A nil
// deriver means StandardPBKDF2.
type PBKDF2Deriver interface {
	// DerivePBKDF2 derives keyLen bytes from password and salt using
	// PBKDF2-SHA-256 with the given iteration count.
	DerivePBKDF2(password string, salt []byte, iterations, keyLen int) ([]byte, error)
}

// PBKDF2Func adapts a plain function to PBKDF2Deriver.
type PBKDF2Func func(password string, salt []byte, iterations, keyLen int) ([]byte, error)

// DerivePBKDF2 implements PBKDF2Deriver.
func (f PBKDF2Func) DerivePBKDF2(password string, salt []byte, iterations, keyLen int) ([]byte, error) {
	if f == nil {
		return StandardPBKDF2{}.DerivePBKDF2(password, salt, iterations, keyLen)
	}
	return f(password, salt, iterations, keyLen)
}

// StandardPBKDF2 is the pure-Go PBKDF2-SHA-256 deriver (crypto/pbkdf2).
type StandardPBKDF2 struct{}

// DerivePBKDF2 implements PBKDF2Deriver using the Go standard library.
func (StandardPBKDF2) DerivePBKDF2(password string, salt []byte, iterations, keyLen int) ([]byte, error) {
	if iterations <= 0 {
		return nil, fmt.Errorf("%w: pbkdf2 iterations must be positive", ErrInvalidSecret)
	}
	if keyLen <= 0 {
		return nil, fmt.Errorf("%w: pbkdf2 key length must be positive", ErrInvalidSecret)
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, iterations, keyLen)
	if err != nil {
		return nil, fmt.Errorf("keyring: pbkdf2: %w", err)
	}
	return key, nil
}

// FixedKEKDeriver is a PBKDF2Deriver that returns an already-derived
// key-encryption key. It exists for the browser flow, where
// WebCryptoPBKDF2 runs asynchronously first and the result is then handed
// to the synchronous wrap/unwrap calls. It must only be used with a KEK
// derived from exactly the passcode, salt, and iteration count of the entry
// being wrapped or unwrapped; it ignores those parameters except for
// checking the requested length. Zero KEK when done.
type FixedKEKDeriver struct {
	// KEK is the pre-derived key-encryption key.
	KEK []byte
}

// DerivePBKDF2 implements PBKDF2Deriver by returning a copy of the fixed KEK.
func (d FixedKEKDeriver) DerivePBKDF2(_ string, _ []byte, _, keyLen int) ([]byte, error) {
	if len(d.KEK) != keyLen {
		return nil, fmt.Errorf("%w: pre-derived KEK is %d bytes, want %d", ErrInvalidSecret, len(d.KEK), keyLen)
	}
	return cloneBytes(d.KEK), nil
}

// deriverOrStandard returns the deriver to use, defaulting to StandardPBKDF2.
func deriverOrStandard(d PBKDF2Deriver) PBKDF2Deriver {
	if d == nil {
		return StandardPBKDF2{}
	}
	return d
}

// DerivePasscodeKEK derives the passcode key-encryption key for an entry
// with the given salt and iteration count. This is the one expensive step of
// an unlock; callers should run it once per unlock, never per save.
func DerivePasscodeKEK(passcode string, salt []byte, iterations int, deriver PBKDF2Deriver) ([]byte, error) {
	if len(salt) == 0 {
		return nil, fmt.Errorf("%w: passcode salt is empty", ErrInvalidSecret)
	}
	return deriverOrStandard(deriver).DerivePBKDF2(passcode, salt, iterations, KeySize)
}

// DeriveRecoveryKEK derives the key-encryption key for a recovery secret.
func DeriveRecoveryKEK(secret []byte) ([]byte, error) {
	if len(secret) != RecoverySecretSize {
		return nil, fmt.Errorf("%w: recovery secret is %d bytes, want %d", ErrInvalidSecret, len(secret), RecoverySecretSize)
	}
	return hkdfSHA256(secret, infoRecovery)
}

// DerivePasskeyKEK derives the key-encryption key for a WebAuthn PRF output.
func DerivePasskeyKEK(prfOutput []byte) ([]byte, error) {
	if len(prfOutput) != PRFOutputSize {
		return nil, fmt.Errorf("%w: passkey PRF output is %d bytes, want %d", ErrInvalidSecret, len(prfOutput), PRFOutputSize)
	}
	return hkdfSHA256(prfOutput, infoPasskey)
}

// Wrapper seals and opens the WDK under a key this package never sees — in
// the browser, a non-extractable WebCrypto AES-GCM key stored in IndexedDB.
// Wrap returns the nonce and ciphertext stored in the wrap entry; Unwrap
// reverses it. Implementations must authenticate aad (AES-GCM additional
// data) so a wrapped key cannot be replayed into a different entry.
type Wrapper interface {
	// Wrap seals plaintext (the WDK) binding aad, returning nonce and
	// ciphertext for storage in the wrap entry.
	Wrap(plaintext, aad []byte) (nonce, ciphertext []byte, err error)
	// Unwrap opens a value produced by Wrap with the same aad.
	Unwrap(nonce, ciphertext, aad []byte) ([]byte, error)
}

// AESGCMWrapper is a Wrapper backed by a raw 32-byte AES-256-GCM key held in
// Go memory. It suits native device keys and tests; the browser should use a
// non-extractable WebCrypto key instead, so the key bytes never enter Go.
type AESGCMWrapper struct {
	// Key is the 32-byte device key. The wrapper does not copy it; zero
	// it when the device key is discarded.
	Key []byte
}

// Wrap implements Wrapper with a fresh random nonce.
func (w AESGCMWrapper) Wrap(plaintext, aad []byte) ([]byte, []byte, error) {
	nonce, err := randomBytes(NonceSize)
	if err != nil {
		return nil, nil, err
	}
	ciphertext, err := sealWithNonce(w.Key, nonce, aad, plaintext)
	if err != nil {
		return nil, nil, err
	}
	return nonce, ciphertext, nil
}

// Unwrap implements Wrapper.
func (w AESGCMWrapper) Unwrap(nonce, ciphertext, aad []byte) ([]byte, error) {
	return openWithNonce(w.Key, nonce, aad, ciphertext)
}

// UnwrapSecret carries the secret for opening one wrap entry. Exactly the
// fields for the entry's kind are read:
//
//   - passcode: Passcode (plus an optional custom Deriver)
//   - recovery: RecoveryCode (the code as typed) or RecoverySecret (raw)
//   - passkey: PRFOutput, and CredentialID when selecting among passkeys
//   - device: Device (the platform Wrapper)
type UnwrapSecret struct {
	// Passcode is the user's passcode (passcode entries).
	Passcode string
	// Deriver optionally overrides passcode key derivation. Nil uses the
	// standard library PBKDF2.
	Deriver PBKDF2Deriver
	// RecoveryCode is a recovery code as typed or scanned (recovery
	// entries). It is decoded with DecodeRecoveryCode.
	RecoveryCode string
	// RecoverySecret is the raw 16-byte recovery secret (recovery
	// entries). Takes precedence over RecoveryCode when set.
	RecoverySecret []byte
	// PRFOutput is the 32-byte WebAuthn PRF output (passkey entries).
	PRFOutput []byte
	// CredentialID selects the passkey entry when the entry ID does not.
	// It is compared against the entry's stored credential ID.
	CredentialID []byte
	// Device is the platform wrapper (device entries).
	Device Wrapper
}

// wrapMaterial seals wk's material under kek with a fresh nonce and returns
// nonce and ciphertext, binding the entry via wrapAAD.
func wrapMaterial(wk *WorkspaceKey, kind WrapKind, entryID string, kek []byte) (nonce, wrapped []byte, err error) {
	nonce, err = randomBytes(NonceSize)
	if err != nil {
		return nil, nil, err
	}
	wrapped, err = sealWithNonce(kek, nonce, wrapAAD(kind, wk.keyID, wk.version, entryID), wk.material)
	if err != nil {
		return nil, nil, err
	}
	return nonce, wrapped, nil
}

// WrapWithPasscode builds a passcode wrap entry for wk. A fresh salt is
// generated and stored in the entry; pass an explicit salt via
// WrapWithPasscodeSalt only when re-deriving for an existing entry. An empty
// passcode is rejected: "no passcode" is represented by having no passcode
// entry, never by an entry anyone can open.
func WrapWithPasscode(wk *WorkspaceKey, passcode string, deriver PBKDF2Deriver) (WrapEntry, error) {
	salt, err := randomBytes(PasscodeSaltSize)
	if err != nil {
		return WrapEntry{}, err
	}
	return wrapWithPasscodeSalt(wk, "", passcode, salt, deriver)
}

// wrapWithPasscodeSalt builds a passcode entry with a caller-supplied salt.
func wrapWithPasscodeSalt(wk *WorkspaceKey, id, passcode string, salt []byte, deriver PBKDF2Deriver) (WrapEntry, error) {
	if wk.Destroyed() {
		return WrapEntry{}, fmt.Errorf("%w: workspace key has been destroyed", ErrInvalidKey)
	}
	if passcode == "" {
		return WrapEntry{}, fmt.Errorf("%w: passcode is empty", ErrInvalidSecret)
	}
	kek, err := DerivePasscodeKEK(passcode, salt, PasscodeIterations, deriver)
	if err != nil {
		return WrapEntry{}, err
	}
	defer clearBytes(kek)
	entry := WrapEntry{Kind: WrapPasscode, ID: id, Salt: cloneBytes(salt), Iterations: PasscodeIterations}
	_, entryID := entry.identity()
	nonce, wrapped, err := wrapMaterial(wk, WrapPasscode, entryID, kek)
	if err != nil {
		return WrapEntry{}, err
	}
	entry.Nonce, entry.Wrapped = nonce, wrapped
	return entry, nil
}

// WrapWithRecoverySecret builds a recovery wrap entry for wk from an
// existing recovery secret.
func WrapWithRecoverySecret(wk *WorkspaceKey, secret []byte) (WrapEntry, error) {
	if wk.Destroyed() {
		return WrapEntry{}, fmt.Errorf("%w: workspace key has been destroyed", ErrInvalidKey)
	}
	kek, err := DeriveRecoveryKEK(secret)
	if err != nil {
		return WrapEntry{}, err
	}
	defer clearBytes(kek)
	entry := WrapEntry{Kind: WrapRecovery}
	_, entryID := entry.identity()
	nonce, wrapped, err := wrapMaterial(wk, WrapRecovery, entryID, kek)
	if err != nil {
		return WrapEntry{}, err
	}
	entry.Nonce, entry.Wrapped = nonce, wrapped
	return entry, nil
}

// WrapWithRecovery generates a fresh recovery secret, builds its wrap entry
// for wk, and returns the recovery code to show the user exactly once. The
// code is not stored anywhere in the entry; if the user loses it, this wrap
// can never be opened again.
func WrapWithRecovery(wk *WorkspaceKey) (entry WrapEntry, code string, err error) {
	code, secret, err := GenerateRecoveryCode()
	if err != nil {
		return WrapEntry{}, "", err
	}
	defer clearBytes(secret)
	entry, err = WrapWithRecoverySecret(wk, secret)
	if err != nil {
		return WrapEntry{}, "", err
	}
	return entry, code, nil
}

// WrapWithPasskey builds a passkey wrap entry for wk. credentialID and
// prfSalt are stored in the entry (neither is secret); prfOutput is used
// only to derive the KEK and is never stored.
func WrapWithPasskey(wk *WorkspaceKey, credentialID, prfSalt, prfOutput []byte) (WrapEntry, error) {
	if wk.Destroyed() {
		return WrapEntry{}, fmt.Errorf("%w: workspace key has been destroyed", ErrInvalidKey)
	}
	if len(credentialID) == 0 {
		return WrapEntry{}, fmt.Errorf("%w: passkey credential id is empty", ErrInvalidSecret)
	}
	if len(prfSalt) == 0 {
		return WrapEntry{}, fmt.Errorf("%w: passkey PRF salt is empty", ErrInvalidSecret)
	}
	kek, err := DerivePasskeyKEK(prfOutput)
	if err != nil {
		return WrapEntry{}, err
	}
	defer clearBytes(kek)
	entry := WrapEntry{
		Kind:         WrapPasskey,
		CredentialID: cloneBytes(credentialID),
		PRFSalt:      cloneBytes(prfSalt),
	}
	_, entryID := entry.identity()
	nonce, wrapped, err := wrapMaterial(wk, WrapPasskey, entryID, kek)
	if err != nil {
		return WrapEntry{}, err
	}
	entry.Nonce, entry.Wrapped = nonce, wrapped
	return entry, nil
}

// WrapWithDevice builds a device wrap entry for wk using the platform
// wrapper. deviceID distinguishes devices (each device adds its own entry
// when it joins the workspace); it may be empty for a single-device setup.
func WrapWithDevice(wk *WorkspaceKey, deviceID string, wrapper Wrapper) (WrapEntry, error) {
	if wk.Destroyed() {
		return WrapEntry{}, fmt.Errorf("%w: workspace key has been destroyed", ErrInvalidKey)
	}
	if wrapper == nil {
		return WrapEntry{}, fmt.Errorf("%w: device wrapper is nil", ErrInvalidSecret)
	}
	entry := WrapEntry{Kind: WrapDevice, ID: deviceID}
	_, entryID := entry.identity()
	nonce, wrapped, err := wrapper.Wrap(wk.material, wrapAAD(WrapDevice, wk.keyID, wk.version, entryID))
	if err != nil {
		return WrapEntry{}, fmt.Errorf("keyring: device wrap: %w", err)
	}
	entry.Nonce, entry.Wrapped = cloneBytes(nonce), cloneBytes(wrapped)
	return entry, nil
}

// unwrapEntry opens one wrap entry belonging to vk with secret. It never
// mutates the entry. Authentication failures return an *AuthenticationError
// wrapped with ErrUnwrapFailed.
func unwrapEntry(vk VersionedKey, entry WrapEntry, secret UnwrapSecret) (*WorkspaceKey, error) {
	_, entryID := entry.identity()
	aad := wrapAAD(entry.Kind, vk.KeyID, vk.Version, entryID)

	var material []byte
	switch entry.Kind {
	case WrapPasscode:
		kek, err := DerivePasscodeKEK(secret.Passcode, entry.Salt, entry.Iterations, secret.Deriver)
		if err != nil {
			return nil, err
		}
		defer clearBytes(kek)
		material, err = openWithNonce(kek, entry.Nonce, aad, entry.Wrapped)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrUnwrapFailed, err)
		}
	case WrapRecovery:
		raw := secret.RecoverySecret
		if raw == nil {
			if secret.RecoveryCode == "" {
				return nil, fmt.Errorf("%w: recovery code is empty", ErrInvalidSecret)
			}
			decoded, err := DecodeRecoveryCode(secret.RecoveryCode)
			if err != nil {
				return nil, err
			}
			defer clearBytes(decoded)
			raw = decoded
		}
		kek, err := DeriveRecoveryKEK(raw)
		if err != nil {
			return nil, err
		}
		defer clearBytes(kek)
		material, err = openWithNonce(kek, entry.Nonce, aad, entry.Wrapped)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrUnwrapFailed, err)
		}
	case WrapPasskey:
		if len(secret.CredentialID) > 0 && !bytesEqual(secret.CredentialID, entry.CredentialID) {
			return nil, fmt.Errorf("%w: passkey credential does not match this entry", ErrUnwrapFailed)
		}
		kek, err := DerivePasskeyKEK(secret.PRFOutput)
		if err != nil {
			return nil, err
		}
		defer clearBytes(kek)
		material, err = openWithNonce(kek, entry.Nonce, aad, entry.Wrapped)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrUnwrapFailed, err)
		}
	case WrapDevice:
		if secret.Device == nil {
			return nil, fmt.Errorf("%w: device wrapper is nil", ErrInvalidSecret)
		}
		opened, err := secret.Device.Unwrap(entry.Nonce, entry.Wrapped, aad)
		if err != nil {
			return nil, fmt.Errorf("%w: device unwrap: %w", ErrUnwrapFailed, err)
		}
		material = opened
	default:
		return nil, fmt.Errorf("%w: unknown wrap kind %q", ErrInvalidKeyring, entry.Kind)
	}
	defer clearBytes(material)
	return workspaceKeyFromMaterial(vk.KeyID, vk.Version, material)
}

// bytesEqual reports whether a and b hold the same bytes.
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	diff := byte(0)
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

// AddWrap stores entry under wk's key version, creating that version in the
// keyring if needed. It verifies the entry actually unwraps to wk (by
// checking identity against wk's version set), rejects a second entry with
// the same kind and ID, and copies the entry. Build entries with the
// WrapWith* constructors using the same wk.
func (kr *Keyring) AddWrap(wk *WorkspaceKey, entry WrapEntry) error {
	if kr == nil {
		return fmt.Errorf("%w: keyring is nil", ErrInvalidKeyring)
	}
	if wk.Destroyed() {
		return fmt.Errorf("%w: workspace key has been destroyed", ErrInvalidKey)
	}
	if err := validateWrapEntry(entry); err != nil {
		return err
	}
	vk := kr.version(wk.version)
	if vk == nil {
		kr.Keys = append(kr.Keys, VersionedKey{KeyID: wk.keyID, Version: wk.version, Wraps: []WrapEntry{}})
		vk = kr.version(wk.version)
	}
	if vk.KeyID != wk.keyID {
		return fmt.Errorf("%w: key id mismatch for version %d", ErrInvalidKeyring, wk.version)
	}
	kind, id := entry.identity()
	if vk.findWrap(kind, id) != nil {
		return fmt.Errorf("%w: %s wrap %q already exists for version %d", ErrDuplicateWrap, kind, id, wk.version)
	}
	// Prove the entry belongs to this key version: an entry sealed for a
	// different KeyID would carry mismatched wrap AAD and could never be
	// unwrapped through this keyring. We cannot open it here (no secret),
	// so the guard above (same version slot) plus constructor use is the
	// contract; Unwrap re-checks by authentication.
	vk.Wraps = append(vk.Wraps, entry.Clone())
	return nil
}

// AddPasscodeWrap builds and stores a passcode wrap entry for wk's version.
func (kr *Keyring) AddPasscodeWrap(wk *WorkspaceKey, passcode string, deriver PBKDF2Deriver) error {
	entry, err := WrapWithPasscode(wk, passcode, deriver)
	if err != nil {
		return err
	}
	return kr.AddWrap(wk, entry)
}

// AddRecoveryWrap generates a recovery code, stores its wrap entry for wk's
// version, and returns the code to show the user once.
func (kr *Keyring) AddRecoveryWrap(wk *WorkspaceKey) (string, error) {
	entry, code, err := WrapWithRecovery(wk)
	if err != nil {
		return "", err
	}
	if err := kr.AddWrap(wk, entry); err != nil {
		return "", err
	}
	return code, nil
}

// AddPasskeyWrap builds and stores a passkey wrap entry for wk's version.
func (kr *Keyring) AddPasskeyWrap(wk *WorkspaceKey, credentialID, prfSalt, prfOutput []byte) error {
	entry, err := WrapWithPasskey(wk, credentialID, prfSalt, prfOutput)
	if err != nil {
		return err
	}
	return kr.AddWrap(wk, entry)
}

// AddDeviceWrap builds and stores a device wrap entry for wk's version.
func (kr *Keyring) AddDeviceWrap(wk *WorkspaceKey, deviceID string, wrapper Wrapper) error {
	entry, err := WrapWithDevice(wk, deviceID, wrapper)
	if err != nil {
		return err
	}
	return kr.AddWrap(wk, entry)
}

// RemoveWrap deletes the wrap entry with the given kind and ID from the
// given key version. It returns ErrWrapNotFound (wrapped) if there is no
// such entry, and ErrVersionNotFound if the version is unknown. Removing
// the last entry of the current version would make the workspace
// unrecoverable; RemoveWrap refuses that unless force is true.
func (kr *Keyring) RemoveWrap(version uint32, kind WrapKind, id string, force bool) error {
	vk := kr.version(version)
	if vk == nil {
		return fmt.Errorf("%w: version %d", ErrVersionNotFound, version)
	}
	for i := range vk.Wraps {
		k, wid := vk.Wraps[i].identity()
		if k != kind || wid != id {
			continue
		}
		if version == kr.CurrentVersion && len(vk.Wraps) == 1 && !force {
			return fmt.Errorf("%w: refusing to remove the last wrap of the current version without force", ErrInvalidKeyring)
		}
		vk.Wraps = append(vk.Wraps[:i], vk.Wraps[i+1:]...)
		return nil
	}
	return fmt.Errorf("%w: %s wrap %q in version %d", ErrWrapNotFound, kind, id, version)
}

// Unwrap opens a wrap entry of the current key version. id selects among
// entries of the same kind ("" for the default; for passkeys the base64
// credential ID also works, or pass secret.CredentialID with the entry's id
// empty only when there is one candidate — prefer an explicit id).
// Unwrap never modifies the keyring, whether it succeeds or fails.
func (kr *Keyring) Unwrap(kind WrapKind, id string, secret UnwrapSecret) (*WorkspaceKey, error) {
	if kr == nil {
		return nil, fmt.Errorf("%w: keyring is nil", ErrInvalidKeyring)
	}
	return kr.UnwrapVersion(kr.CurrentVersion, kind, id, secret)
}

// UnwrapVersion opens a wrap entry of a specific key version, including
// retained old versions after a rotation. It never modifies the keyring.
func (kr *Keyring) UnwrapVersion(version uint32, kind WrapKind, id string, secret UnwrapSecret) (*WorkspaceKey, error) {
	vk := kr.version(version)
	if vk == nil {
		return nil, fmt.Errorf("%w: version %d", ErrVersionNotFound, version)
	}
	entry := vk.findWrap(kind, id)
	if entry == nil && kind == WrapPasskey && len(secret.CredentialID) > 0 {
		// Allow selecting a passkey entry by credential directly.
		for i := range vk.Wraps {
			if vk.Wraps[i].Kind == WrapPasskey && bytesEqual(vk.Wraps[i].CredentialID, secret.CredentialID) {
				entry = &vk.Wraps[i]
				break
			}
		}
	}
	if entry == nil {
		return nil, fmt.Errorf("%w: %s wrap %q in version %d", ErrWrapNotFound, kind, id, version)
	}
	return unwrapEntry(vk.Clone(), entry.Clone(), secret)
}

// UnwrapEntry opens a single detached wrap entry against an explicit key
// identity. It is the seam for opening an entry before the rest of the
// keyring has been loaded.
func UnwrapEntry(keyID KeyID, version uint32, entry WrapEntry, secret UnwrapSecret) (*WorkspaceKey, error) {
	return unwrapEntry(VersionedKey{KeyID: keyID, Version: version}, entry, secret)
}

// Rewrap replaces the wrap entry with the same kind and ID as entry under
// wk's version, or adds it if none exists. The WDK itself is unchanged:
// changing a passcode is a rewrap, not a rotation, so no data is
// re-encrypted. Build entry with a WrapWith* constructor using the same wk.
func (kr *Keyring) Rewrap(wk *WorkspaceKey, entry WrapEntry) error {
	if kr == nil {
		return fmt.Errorf("%w: keyring is nil", ErrInvalidKeyring)
	}
	if wk.Destroyed() {
		return fmt.Errorf("%w: workspace key has been destroyed", ErrInvalidKey)
	}
	if err := validateWrapEntry(entry); err != nil {
		return err
	}
	vk := kr.version(wk.version)
	if vk == nil {
		return kr.AddWrap(wk, entry)
	}
	if vk.KeyID != wk.keyID {
		return fmt.Errorf("%w: key id mismatch for version %d", ErrInvalidKeyring, wk.version)
	}
	kind, id := entry.identity()
	existing := vk.findWrap(kind, id)
	if existing == nil {
		vk.Wraps = append(vk.Wraps, entry.Clone())
		return nil
	}
	*existing = entry.Clone()
	return nil
}

// RewrapPasscode changes the passcode: it builds a fresh passcode wrap entry
// (new salt) for the same WDK and replaces the old one. The WDK bytes — and
// therefore every subkey and all existing ciphertext — are unchanged.
func (kr *Keyring) RewrapPasscode(wk *WorkspaceKey, newPasscode string, deriver PBKDF2Deriver) error {
	entry, err := WrapWithPasscode(wk, newPasscode, deriver)
	if err != nil {
		return err
	}
	return kr.Rewrap(wk, entry)
}

// Rotate generates the next key version and makes it current. The old
// version stays in the keyring so existing ciphertext remains readable. The
// returned key has no wrap entries yet: the caller must add wraps for it
// (AddPasscodeWrap, AddRecoveryWrap, ...) and persist the keyring before
// discarding either key.
func (kr *Keyring) Rotate() (*WorkspaceKey, error) {
	if kr == nil || len(kr.Keys) == 0 {
		return nil, fmt.Errorf("%w: cannot rotate an empty keyring", ErrInvalidKeyring)
	}
	var maxVersion uint32
	for _, k := range kr.Keys {
		if k.Version > maxVersion {
			maxVersion = k.Version
		}
	}
	if maxVersion == ^uint32(0) {
		return nil, fmt.Errorf("%w: key version overflow", ErrInvalidKeyring)
	}
	next, err := NewWorkspaceKey()
	if err != nil {
		return nil, err
	}
	next.version = maxVersion + 1
	kr.Keys = append(kr.Keys, VersionedKey{
		KeyID:   next.keyID,
		Version: next.version,
		Wraps:   []WrapEntry{},
	})
	kr.CurrentVersion = next.version
	return next, nil
}

// RotateRetainingWraps rotates and immediately re-creates the wraps the
// caller supplies via add (called with the new key). It is a convenience so
// "rotate" flows cannot forget the wrap step: after it returns without
// error, the new current version has whatever wraps add installed, and it
// fails (leaving CurrentVersion unchanged) if add errors or adds nothing.
func (kr *Keyring) RotateRetainingWraps(add func(*Keyring, *WorkspaceKey) error) (*WorkspaceKey, error) {
	if add == nil {
		return nil, fmt.Errorf("%w: rotate requires a wrap installer", ErrInvalidKeyring)
	}
	previous := kr.CurrentVersion
	next, err := kr.Rotate()
	if err != nil {
		return nil, err
	}
	rollback := func(cause error) (*WorkspaceKey, error) {
		// Drop the unwrapped new version; nothing was sealed under it yet.
		for i := range kr.Keys {
			if kr.Keys[i].Version == next.version {
				kr.Keys = append(kr.Keys[:i], kr.Keys[i+1:]...)
				break
			}
		}
		kr.CurrentVersion = previous
		next.Destroy()
		return nil, cause
	}
	if err := add(kr, next); err != nil {
		return rollback(fmt.Errorf("keyring: install wraps for rotated key: %w", err))
	}
	if vk := kr.version(next.version); vk == nil || len(vk.Wraps) == 0 {
		return rollback(fmt.Errorf("%w: rotated key has no wrap entries", ErrInvalidKeyring))
	}
	return next, nil
}

// IsUnwrapError reports whether err indicates the secret did not open the
// wrap (wrong passcode, wrong recovery code, wrong authenticator, wrong
// device), as opposed to a malformed keyring or missing entry.
func IsUnwrapError(err error) bool {
	return errors.Is(err, ErrUnwrapFailed) || errors.Is(err, ErrAuthenticationFailed)
}
