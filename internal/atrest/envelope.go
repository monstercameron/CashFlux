// SPDX-License-Identifier: MIT

package atrest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/keyring"
)

const (
	// EnvelopeFormat identifies this JSON record format.
	EnvelopeFormat = "cashflux-atrest"
	// EnvelopeVersion is the current record envelope format version.
	EnvelopeVersion = 1
)

var (
	// ErrInvalidEnvelope indicates malformed or unsupported envelope data.
	ErrInvalidEnvelope = errors.New("atrest: invalid envelope")
	// ErrKeyMismatch indicates the envelope names a different workspace key.
	ErrKeyMismatch = errors.New("atrest: key identity mismatch")
	// ErrInvalidStoredValue indicates bytes that are neither a valid envelope
	// nor valid legacy JSON.
	ErrInvalidStoredValue = errors.New("atrest: invalid stored value")
)

// Envelope is the versioned JSON representation of one encrypted record.
// Scope, workspace, and record key remain outside the envelope and must be
// supplied unchanged when opening it.
type Envelope struct {
	// Format is the EnvelopeFormat discriminator.
	Format string `json:"format"`
	// Version is the envelope schema version.
	Version int `json:"version"`
	// KeyID identifies the workspace key used to seal the record.
	KeyID keyring.KeyID `json:"keyId"`
	// KeyVersion is the version of the workspace key used to seal the record.
	KeyVersion uint32 `json:"keyVersion"`
	// Nonce is the AES-GCM nonce, stored separately from the ciphertext.
	Nonce []byte `json:"nonce"`
	// Ciphertext is the encrypted value including its authentication tag.
	Ciphertext []byte `json:"ciphertext"`
}

// Encode serializes e as the canonical JSON envelope representation.
func (e Envelope) Encode() ([]byte, error) {
	if err := e.validate(); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("atrest: encode envelope: %w", err)
	}
	return raw, nil
}

// SealRecord encrypts plaintext under wk's data subkey and binds it to scope,
// workspaceID, recordKey, and the key version in authenticated data.
func SealRecord(wk *keyring.WorkspaceKey, scope keyring.Scope, workspaceID, recordKey string, plaintext []byte) (Envelope, error) {
	if wk == nil || wk.Destroyed() {
		return Envelope{}, fmt.Errorf("atrest: seal record: %w", keyring.ErrInvalidKey)
	}
	if workspaceID == "" || recordKey == "" {
		return Envelope{}, fmt.Errorf("atrest: seal record: %w: workspace and record key are required", ErrInvalidEnvelope)
	}
	dataKey, err := wk.DataKey()
	if err != nil {
		return Envelope{}, fmt.Errorf("atrest: derive data key: %w", err)
	}
	defer zero(dataKey)
	aad := keyring.RecordAAD(scope, workspaceID, recordKey, wk.Version())
	sealed, err := keyring.Seal(dataKey, aad, plaintext)
	if err != nil {
		return Envelope{}, fmt.Errorf("atrest: seal record: %w", err)
	}
	e := Envelope{
		Format:     EnvelopeFormat,
		Version:    EnvelopeVersion,
		KeyID:      wk.KeyID(),
		KeyVersion: wk.Version(),
		Nonce:      append([]byte(nil), sealed[:keyring.NonceSize]...),
		Ciphertext: append([]byte(nil), sealed[keyring.NonceSize:]...),
	}
	return e, nil
}

// OpenRecord authenticates and decrypts e only when it belongs to the given
// workspace key, scope, workspace, and record key.
func OpenRecord(wk *keyring.WorkspaceKey, scope keyring.Scope, workspaceID, recordKey string, e Envelope) ([]byte, error) {
	if err := e.validate(); err != nil {
		return nil, err
	}
	if wk == nil || wk.Destroyed() {
		return nil, fmt.Errorf("atrest: open record: %w", keyring.ErrInvalidKey)
	}
	if e.KeyID != wk.KeyID() || e.KeyVersion != wk.Version() {
		return nil, fmt.Errorf("atrest: open record: %w", ErrKeyMismatch)
	}
	dataKey, err := wk.DataKey()
	if err != nil {
		return nil, fmt.Errorf("atrest: derive data key: %w", err)
	}
	defer zero(dataKey)
	sealed := make([]byte, 0, len(e.Nonce)+len(e.Ciphertext))
	sealed = append(sealed, e.Nonce...)
	sealed = append(sealed, e.Ciphertext...)
	plain, err := keyring.Open(dataKey, keyring.RecordAAD(scope, workspaceID, recordKey, e.KeyVersion), sealed)
	if err != nil {
		return nil, fmt.Errorf("atrest: open record: %w", err)
	}
	return plain, nil
}

func (e Envelope) validate() error {
	if e.Format != EnvelopeFormat {
		return fmt.Errorf("%w: format is %q", ErrInvalidEnvelope, e.Format)
	}
	if e.Version != EnvelopeVersion {
		return fmt.Errorf("%w: unsupported version %d", ErrInvalidEnvelope, e.Version)
	}
	if e.KeyID.IsZero() || e.KeyVersion == 0 {
		return fmt.Errorf("%w: key identity is incomplete", ErrInvalidEnvelope)
	}
	if len(e.Nonce) != keyring.NonceSize {
		return fmt.Errorf("%w: nonce is %d bytes, want %d", ErrInvalidEnvelope, len(e.Nonce), keyring.NonceSize)
	}
	if len(e.Ciphertext) < 16 {
		return fmt.Errorf("%w: ciphertext is shorter than an AES-GCM tag", ErrInvalidEnvelope)
	}
	return nil
}

// StoredKind identifies how DecodeStored interpreted persisted bytes.
type StoredKind uint8

const (
	// StoredEnvelope means the bytes contain a current encrypted record.
	StoredEnvelope StoredKind = iota + 1
	// StoredLegacyJSON means the bytes are valid legacy plaintext JSON and
	// should be encrypted before the old value is removed.
	StoredLegacyJSON
)

// StoredValue is either an encrypted envelope or legacy plaintext JSON.
type StoredValue struct {
	// Kind says whether Envelope or Legacy contains the decoded value.
	Kind StoredKind
	// Envelope is non-nil when Kind is StoredEnvelope.
	Envelope *Envelope
	// Legacy is a copy of valid plaintext JSON when Kind is StoredLegacyJSON.
	Legacy []byte
}

// DecodeStored recognizes an encrypted envelope or a legacy plaintext JSON
// value. It never decrypts or mutates the input; callers must durably save a
// replacement before deleting legacy bytes.
func DecodeStored(raw []byte) (StoredValue, error) {
	if !json.Valid(raw) {
		return StoredValue{}, fmt.Errorf("%w: bytes are not valid JSON", ErrInvalidStoredValue)
	}
	var marker struct {
		Format string `json:"format"`
	}
	if err := json.Unmarshal(raw, &marker); err != nil {
		return StoredValue{}, fmt.Errorf("%w: inspect JSON: %w", ErrInvalidStoredValue, err)
	}
	if marker.Format != EnvelopeFormat {
		return StoredValue{Kind: StoredLegacyJSON, Legacy: bytes.Clone(raw)}, nil
	}
	var e Envelope
	if err := json.Unmarshal(raw, &e); err != nil {
		return StoredValue{}, fmt.Errorf("%w: decode envelope: %w", ErrInvalidEnvelope, err)
	}
	if err := e.validate(); err != nil {
		return StoredValue{}, err
	}
	return StoredValue{Kind: StoredEnvelope, Envelope: &e}, nil
}

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
