// SPDX-License-Identifier: MIT

package keyring

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

// MaxKeyringSize bounds keyring blobs accepted by Parse. A real keyring is
// a few kilobytes; the bound keeps hostile blobs from forcing large
// allocations.
const MaxKeyringSize = 1 << 20

// MaxKeyringVersions bounds the number of key versions accepted in one blob.
const MaxKeyringVersions = 64

// MaxWrapsPerVersion bounds the number of unlock wraps accepted per key version.
const MaxWrapsPerVersion = 32

// MaxFieldsPerObject bounds object fields inspected while preflighting a blob.
const MaxFieldsPerObject = 16

// MaxKeyringJSONDepth bounds nested JSON containers in a keyring blob.
const MaxKeyringJSONDepth = 64

// MaxCredentialIDSize bounds stored WebAuthn credential IDs.
const MaxCredentialIDSize = 1024

// wrapCiphertextSize is the expected wrapped size for keys wrapped by this
// package: the 32-byte WDK plus the 16-byte GCM tag.
const wrapCiphertextSize = KeySize + 16

// Marshal encodes the keyring in its canonical form: JSON with key versions
// sorted ascending and wrap entries sorted by (kind, ID). Equal keyrings
// marshal to identical bytes, which makes change-notification and
// compare-and-swap on the server straightforward. Marshal validates first
// and returns an error instead of writing a blob Parse would reject. The
// receiver is not modified. wk must be the current WDK; the metadata is
// authenticated under its dedicated HKDF subkey. Unsigned v1 blobs are no
// longer accepted and must never be silently upgraded from untrusted storage.
func (kr *Keyring) Marshal(wk *WorkspaceKey) ([]byte, error) {
	if kr == nil {
		return nil, fmt.Errorf("%w: keyring is nil", ErrInvalidKeyring)
	}
	canonical := kr.canonical()
	if err := canonical.Validate(); err != nil {
		return nil, err
	}
	if err := canonical.checkCurrentKey(wk); err != nil {
		return nil, err
	}
	mac, err := canonical.metadataMAC(wk)
	if err != nil {
		return nil, err
	}
	canonical.MetadataMAC = mac
	out, err := json.Marshal(canonical)
	if err != nil {
		return nil, fmt.Errorf("keyring: marshal: %w", err)
	}
	if len(out) > MaxKeyringSize {
		return nil, fmt.Errorf("%w: encoded keyring exceeds %d bytes", ErrInvalidKeyring, MaxKeyringSize)
	}
	return out, nil
}

// canonical returns a sorted deep copy of the keyring.
func (kr *Keyring) canonical() *Keyring {
	out := kr.Clone()
	sort.Slice(out.Keys, func(i, j int) bool { return out.Keys[i].Version < out.Keys[j].Version })
	for i := range out.Keys {
		wraps := out.Keys[i].Wraps
		sort.SliceStable(wraps, func(a, b int) bool {
			ka, ia := wraps[a].identity()
			kb, ib := wraps[b].identity()
			if ka != kb {
				return ka < kb
			}
			return ia < ib
		})
		if wraps == nil {
			out.Keys[i].Wraps = []WrapEntry{}
		}
	}
	return out
}

// Parse decodes a keyring blob produced by Marshal. It is strict: unknown
// JSON fields, trailing data, unknown format versions, duplicate or invalid
// entries, oversized blobs, invalid metadata MACs and rollback below highestSeen
// are rejected. wk must be the current WDK, obtained from trusted memory or
// Unlock. Persist highestSeen per scope/workspace after successful verification,
// durably before acknowledging use of the new version. Zero means first use.
// On error no partial keyring is returned.
func Parse(data []byte, wk *WorkspaceKey, highestSeen uint32) (*Keyring, error) {
	kr, err := parseUntrusted(data, highestSeen)
	if err != nil {
		return nil, err
	}
	if err := kr.checkCurrentKey(wk); err != nil {
		return nil, err
	}
	mac, err := kr.metadataMAC(wk)
	if err != nil {
		return nil, err
	}
	if !hmac.Equal(mac, kr.MetadataMAC) {
		return nil, &AuthenticationError{Operation: "keyring metadata"}
	}
	return kr, nil
}

// Unlock bootstraps a locked keyring without exposing unauthenticated metadata.
// It unwraps a candidate current WDK, then authenticates the whole blob and
// version floor before returning either value. No partial result escapes on
// failure. Treat an error as unreadable data, never as permission to overwrite.
func Unlock(data []byte, highestSeen uint32, kind WrapKind, id string, secret UnwrapSecret) (*Keyring, *WorkspaceKey, error) {
	kr, err := parseUntrusted(data, highestSeen)
	if err != nil {
		return nil, nil, err
	}
	wk, err := kr.Unwrap(kind, id, secret)
	if err != nil {
		return nil, nil, err
	}
	verified, err := Parse(data, wk, highestSeen)
	if err != nil {
		wk.Destroy()
		return nil, nil, err
	}
	return verified, wk, nil
}

// parseUntrusted is private: callers must authenticate before using metadata.
func parseUntrusted(data []byte, highestSeen uint32) (*Keyring, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: keyring blob is empty", ErrInvalidKeyring)
	}
	if len(data) > MaxKeyringSize {
		return nil, fmt.Errorf("%w: keyring blob is %d bytes, limit is %d", ErrInvalidKeyring, len(data), MaxKeyringSize)
	}
	if err := preflightKeyringJSON(data); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var kr Keyring
	if err := dec.Decode(&kr); err != nil {
		return nil, fmt.Errorf("%w: decode keyring: %v", ErrInvalidKeyring, err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("%w: trailing data after keyring blob", ErrInvalidKeyring)
	}
	if err := kr.Validate(); err != nil {
		return nil, err
	}
	if kr.CurrentVersion < highestSeen {
		return nil, fmt.Errorf("%w: current version %d is below highest seen %d", ErrRollback, kr.CurrentVersion, highestSeen)
	}
	if len(kr.MetadataMAC) != sha256.Size {
		return nil, &AuthenticationError{Operation: "keyring metadata"}
	}
	canonical, err := json.Marshal(kr.canonical())
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(data, canonical) {
		return nil, fmt.Errorf("%w: keyring blob is not canonically encoded", ErrInvalidKeyring)
	}
	return &kr, nil
}

// metadataMAC covers the canonical format, current version, entire version
// list, key IDs/checks, wrap identities and wrap contents; only the MAC itself
// is omitted. Encoding a fixed nil MAC avoids a second schema for signing.
func (kr *Keyring) metadataMAC(wk *WorkspaceKey) ([]byte, error) {
	key, err := wk.DeriveSubkey(SubkeyMetadata)
	if err != nil {
		return nil, err
	}
	defer clearBytes(key)
	canonical := kr.canonical()
	canonical.MetadataMAC = nil
	data, err := json.Marshal(canonical)
	if err != nil {
		return nil, fmt.Errorf("keyring: encode metadata for MAC: %w", err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("cfv2/keyring-metadata/v2"))
	mac.Write(data)
	return mac.Sum(nil), nil
}

func (wk *WorkspaceKey) keyCheck() ([]byte, error) {
	key, err := wk.DeriveSubkey(SubkeyKeyCheck)
	if err != nil {
		return nil, err
	}
	defer clearBytes(key)
	var version [4]byte
	binary.BigEndian.PutUint32(version[:], wk.version)
	mac := hmac.New(sha256.New, key)
	mac.Write(joinLengthPrefixed([]byte("cfv2/key-check/v2"), wk.keyID[:], version[:]))
	return mac.Sum(nil), nil
}

func (kr *Keyring) checkCurrentKey(wk *WorkspaceKey) error {
	if wk.Destroyed() {
		return fmt.Errorf("%w: current workspace key is unavailable", ErrInvalidKey)
	}
	current := kr.Current()
	if current == nil || current.Version != wk.version || current.KeyID != wk.keyID {
		return &AuthenticationError{Operation: "keyring key identity"}
	}
	check, err := wk.keyCheck()
	if err != nil {
		return err
	}
	if !hmac.Equal(check, current.KeyCheck) {
		return &AuthenticationError{Operation: "keyring key check"}
	}
	return nil
}

// preflightKeyringJSON bounds structural work before encoding/json allocates
// key and wrap slices or skips unknown nested values. The regular decoder
// remains responsible for syntax and schema validation.
func preflightKeyringJSON(data []byte) error {
	type container struct {
		kind       byte
		count      int
		hasContent bool
		arrayLimit int
	}
	stack := make([]container, 0, 8)
	inString, escaped := false, false
	lastProperty := ""
	for i := 0; i < len(data); i++ {
		b := data[i]
		if inString {
			if escaped {
				escaped = false
			} else if b == '\\' {
				escaped = true
			} else if b == '"' {
				inString = false
			}
			continue
		}
		if b == '"' {
			inString = true
			start := i + 1
			propertyCandidate := len(stack) > 0 && stack[len(stack)-1].kind == '{'
			for i++; i < len(data); i++ {
				if data[i] == '\\' {
					i++
				} else if data[i] == '"' {
					break
				}
			}
			if i < len(data) {
				property := string(data[start:i])
				j := i + 1
				for j < len(data) && (data[j] == ' ' || data[j] == '\n' || data[j] == '\r' || data[j] == '\t') {
					j++
				}
				if propertyCandidate && j < len(data) && data[j] == ':' {
					lastProperty = property
				} else {
					lastProperty = ""
				}
			}
			inString = false
			continue
		}
		switch b {
		case '{', '[':
			if len(stack) > 0 {
				stack[len(stack)-1].hasContent = true
			}
			if len(stack)+1 > MaxKeyringJSONDepth {
				return fmt.Errorf("%w: JSON nesting exceeds %d", ErrInvalidKeyring, MaxKeyringJSONDepth)
			}
			stack = append(stack, container{kind: b})
			if b == '[' {
				limit := MaxWrapsPerVersion
				if len(stack) == 2 && lastProperty == "keys" {
					limit = MaxKeyringVersions
				} else if len(stack) == 4 && lastProperty == "wraps" {
					limit = MaxWrapsPerVersion
				}
				stack[len(stack)-1].arrayLimit = limit
			}
			lastProperty = ""
		case ',', ':':
			if len(stack) > 0 {
				c := &stack[len(stack)-1]
				if b == ',' {
					c.count++
					c.hasContent = true
					if c.kind == '{' && c.count+1 > MaxFieldsPerObject {
						return fmt.Errorf("%w: object has more than %d fields", ErrInvalidKeyring, MaxFieldsPerObject)
					}
				}
			}
			if b == ':' {
				// Preserve the member name until its value container opens.
			}
		case '}', ']':
			if len(stack) == 0 {
				continue
			}
			c := stack[len(stack)-1]
			if c.kind == '[' && c.hasContent {
				if c.count+1 > c.arrayLimit {
					limit := c.arrayLimit
					return fmt.Errorf("%w: array has more than %d entries", ErrInvalidKeyring, limit)
				}
			}
			stack = stack[:len(stack)-1]
		}
	}
	return nil
}

// Validate checks the keyring's structural invariants: known format
// version, at least one key version, unique nonzero versions and key IDs, a
// current version that exists and has at least one wrap entry, and
// well-formed unique wrap entries. It does not (and cannot) check that any
// entry can actually be unwrapped.
func (kr *Keyring) Validate() error {
	if kr == nil {
		return fmt.Errorf("%w: keyring is nil", ErrInvalidKeyring)
	}
	if kr.FormatVersion != FormatVersion {
		return fmt.Errorf("%w: format version %d, this build understands %d", ErrUnsupportedFormat, kr.FormatVersion, FormatVersion)
	}
	if len(kr.Keys) == 0 {
		return fmt.Errorf("%w: keyring has no key versions", ErrInvalidKeyring)
	}
	if len(kr.Keys) > MaxKeyringVersions {
		return fmt.Errorf("%w: too many key versions", ErrInvalidKeyring)
	}
	seenVersions := make(map[uint32]bool, len(kr.Keys))
	seenIDs := make(map[KeyID]bool, len(kr.Keys))
	var current *VersionedKey
	for i := range kr.Keys {
		k := &kr.Keys[i]
		if k.Version == 0 {
			return fmt.Errorf("%w: key version is zero", ErrInvalidKeyring)
		}
		if k.Version > kr.CurrentVersion {
			return fmt.Errorf("%w: current version is below retained version %d", ErrRollback, k.Version)
		}
		if len(k.KeyCheck) != sha256.Size {
			return fmt.Errorf("%w: version %d has an invalid key check", ErrInvalidKeyring, k.Version)
		}
		if len(k.Wraps) > MaxWrapsPerVersion {
			return fmt.Errorf("%w: version %d has too many wraps", ErrInvalidKeyring, k.Version)
		}
		if seenVersions[k.Version] {
			return fmt.Errorf("%w: duplicate key version %d", ErrInvalidKeyring, k.Version)
		}
		seenVersions[k.Version] = true
		if k.KeyID.IsZero() {
			return fmt.Errorf("%w: version %d has a zero key id", ErrInvalidKeyring, k.Version)
		}
		if seenIDs[k.KeyID] {
			return fmt.Errorf("%w: duplicate key id for version %d", ErrInvalidKeyring, k.Version)
		}
		seenIDs[k.KeyID] = true
		if k.Version == kr.CurrentVersion {
			current = k
		}
		seenWraps := make(map[string]bool, len(k.Wraps))
		for _, w := range k.Wraps {
			if err := validateWrapEntry(w); err != nil {
				return fmt.Errorf("keyring: version %d: %w", k.Version, err)
			}
			kind, id := w.identity()
			key := string(kind) + "\x00" + id
			if seenWraps[key] {
				return fmt.Errorf("%w: duplicate %s wrap %q in version %d", ErrInvalidKeyring, kind, id, k.Version)
			}
			seenWraps[key] = true
		}
	}
	if kr.CurrentVersion == 0 {
		return fmt.Errorf("%w: current version is zero", ErrInvalidKeyring)
	}
	if current == nil {
		return fmt.Errorf("%w: current version %d is not in the keyring", ErrInvalidKeyring, kr.CurrentVersion)
	}
	if len(current.Wraps) == 0 {
		return fmt.Errorf("%w: current version %d has no wrap entries and could never be unlocked", ErrInvalidKeyring, kr.CurrentVersion)
	}
	return nil
}

// validateWrapEntry checks one wrap entry's fields for its kind.
func validateWrapEntry(e WrapEntry) error {
	if !e.Kind.Valid() {
		return fmt.Errorf("%w: unknown wrap kind %q", ErrInvalidKeyring, e.Kind)
	}
	if len(e.Wrapped) == 0 {
		return fmt.Errorf("%w: %s wrap has no wrapped key", ErrInvalidKeyring, e.Kind)
	}
	switch e.Kind {
	case WrapPasscode:
		if len(e.Salt) != PasscodeSaltSize {
			return fmt.Errorf("%w: passcode salt is %d bytes, want %d", ErrInvalidKeyring, len(e.Salt), PasscodeSaltSize)
		}
		if e.Iterations != PasscodeIterations {
			return fmt.Errorf("%w: passcode iterations is %d, want %d", ErrInvalidKeyring, e.Iterations, PasscodeIterations)
		}
		if len(e.CredentialID) != 0 || len(e.PRFSalt) != 0 {
			return fmt.Errorf("%w: passcode wrap carries passkey fields", ErrInvalidKeyring)
		}
		if len(e.Nonce) != NonceSize {
			return fmt.Errorf("%w: passcode nonce is %d bytes, want %d", ErrInvalidKeyring, len(e.Nonce), NonceSize)
		}
		if len(e.Wrapped) != wrapCiphertextSize {
			return fmt.Errorf("%w: passcode wrapped key is %d bytes, want %d", ErrInvalidKeyring, len(e.Wrapped), wrapCiphertextSize)
		}
	case WrapRecovery:
		if len(e.Salt) != 0 || e.Iterations != 0 || len(e.CredentialID) != 0 || len(e.PRFSalt) != 0 {
			return fmt.Errorf("%w: recovery wrap carries fields for another kind", ErrInvalidKeyring)
		}
		if len(e.Nonce) != NonceSize {
			return fmt.Errorf("%w: recovery nonce is %d bytes, want %d", ErrInvalidKeyring, len(e.Nonce), NonceSize)
		}
		if len(e.Wrapped) != wrapCiphertextSize {
			return fmt.Errorf("%w: recovery wrapped key is %d bytes, want %d", ErrInvalidKeyring, len(e.Wrapped), wrapCiphertextSize)
		}
	case WrapPasskey:
		if len(e.CredentialID) == 0 || len(e.CredentialID) > MaxCredentialIDSize {
			return fmt.Errorf("%w: passkey credential id is %d bytes", ErrInvalidKeyring, len(e.CredentialID))
		}
		if len(e.PRFSalt) < 16 || len(e.PRFSalt) > 64 {
			return fmt.Errorf("%w: passkey PRF salt is %d bytes, want 16 to 64", ErrInvalidKeyring, len(e.PRFSalt))
		}
		if len(e.Salt) != 0 || e.Iterations != 0 {
			return fmt.Errorf("%w: passkey wrap carries passcode fields", ErrInvalidKeyring)
		}
		if len(e.Nonce) != NonceSize {
			return fmt.Errorf("%w: passkey nonce is %d bytes, want %d", ErrInvalidKeyring, len(e.Nonce), NonceSize)
		}
		if len(e.Wrapped) != wrapCiphertextSize {
			return fmt.Errorf("%w: passkey wrapped key is %d bytes, want %d", ErrInvalidKeyring, len(e.Wrapped), wrapCiphertextSize)
		}
	case WrapDevice:
		if len(e.Salt) != 0 || e.Iterations != 0 || len(e.CredentialID) != 0 || len(e.PRFSalt) != 0 {
			return fmt.Errorf("%w: device wrap carries fields for another kind", ErrInvalidKeyring)
		}
		// The device Wrapper is the platform's, so its nonce and
		// ciphertext sizes are its own contract; only bound them so a
		// hostile blob stays small. (AESGCMWrapper yields the usual
		// 12-byte nonce and 48-byte ciphertext.)
		if len(e.Nonce) > 64 {
			return fmt.Errorf("%w: device nonce is %d bytes", ErrInvalidKeyring, len(e.Nonce))
		}
		if len(e.Wrapped) > 4096 {
			return fmt.Errorf("%w: device wrapped key is %d bytes", ErrInvalidKeyring, len(e.Wrapped))
		}
	}
	return nil
}
