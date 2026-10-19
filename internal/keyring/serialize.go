// SPDX-License-Identifier: MIT

package keyring

import (
	"bytes"
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
// receiver is not modified.
func (kr *Keyring) Marshal() ([]byte, error) {
	if kr == nil {
		return nil, fmt.Errorf("%w: keyring is nil", ErrInvalidKeyring)
	}
	canonical := kr.canonical()
	if err := canonical.Validate(); err != nil {
		return nil, err
	}
	out, err := json.Marshal(canonical)
	if err != nil {
		return nil, fmt.Errorf("keyring: marshal: %w", err)
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
// entries, and oversized blobs are all rejected. On error no partial
// keyring is returned.
func Parse(data []byte) (*Keyring, error) {
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
	canonical, err := kr.Marshal()
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(data, canonical) {
		return nil, fmt.Errorf("%w: keyring blob is not canonically encoded", ErrInvalidKeyring)
	}
	return &kr, nil
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
	seenVersions := make(map[uint32]bool, len(kr.Keys))
	seenIDs := make(map[KeyID]bool, len(kr.Keys))
	var current *VersionedKey
	for i := range kr.Keys {
		k := &kr.Keys[i]
		if k.Version == 0 {
			return fmt.Errorf("%w: key version is zero", ErrInvalidKeyring)
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
