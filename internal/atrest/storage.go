package atrest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// MigrationPrefix reserves temporary, encrypted migration records. Originals
// remain intact until these records have committed and been read back.
const MigrationPrefix = "__cashflux_sealed_migration__:"

var (
	// ErrPersistenceForbidden rejects attempts to persist transient secrets.
	ErrPersistenceForbidden = errors.New("atrest: this key must remain in memory")
	// ErrRecordChanged means a conditional migration would replace newer data.
	ErrRecordChanged = errors.New("atrest: stored record changed")
	// ErrPlaintextRecord means a sensitive legacy record needs migration.
	ErrPlaintextRecord = errors.New("atrest: sensitive record has not been migrated")
)

// RecordBackend provides durable records. Replace and Delete compare the current
// value with expected atomically; nil expected means the key must be absent.
// Success must mean the storage transaction has committed, not merely issued.
type RecordBackend interface {
	Read(key string) (value string, present bool, err error)
	Keys() ([]string, error)
	Replace(key, value string, expected *string) error
	Delete(key string, expected *string) error
}

// RecordCipher binds encrypted records to a fixed scope, workspace and key name.
// KeyService implements this interface.
type RecordCipher interface {
	Seal(recordKey string, plaintext []byte) (Envelope, error)
	Open(recordKey string, envelope Envelope) ([]byte, error)
}

// Storage enforces the sensitivity catalogue over a durable backend. A Storage
// and its cipher belong to one immutable scope for their entire lifetime.
type Storage struct {
	// Backend is the durable, conditional record store.
	Backend RecordBackend
	// Cipher owns the key for this store's immutable scope and workspace.
	Cipher RecordCipher
}

// IsSealed recognizes the envelope discriminator, including malformed envelopes
// which must never be reinterpreted as a new legacy plaintext value.
func IsSealed(value string) bool {
	var header struct {
		Format string `json:"format"`
	}
	if json.Valid([]byte(value)) {
		return json.Unmarshal([]byte(value), &header) == nil && strings.HasPrefix(header.Format, EnvelopeFormat)
	}
	return damagedEnvelopeMarker.MatchString(value)
}

var damagedEnvelopeMarker = regexp.MustCompile(`"format"\s*:\s*"cashflux-atrest`)

// Decode opens a stored value. Sensitive plaintext is an error, never an empty
// dataset. Non-JSON legacy strings are handled by Migrate, without losing bytes.
func (s Storage) Decode(key, value string) (string, error) {
	if Classify(key) == ShouldNotPersist {
		return "", ErrPersistenceForbidden
	}
	if Classify(key) == MayStayPlain {
		return value, nil
	}
	if !IsSealed(value) {
		return "", ErrPlaintextRecord
	}
	e, err := DecodeStored([]byte(value))
	if err != nil {
		return "", err
	}
	if e.Kind != StoredEnvelope {
		return "", ErrInvalidEnvelope
	}
	if s.Cipher == nil {
		return "", ErrLocked
	}
	plain, err := s.Cipher.Open(key, *e.Envelope)
	if err != nil {
		return "", fmt.Errorf("open protected record: %w", err)
	}
	return string(plain), nil
}

// Encode prepares a value for persistence; even an existing envelope supplied by
// a caller is treated as payload, so callers cannot bypass record-key binding.
func (s Storage) Encode(key, value string) (string, error) {
	switch Classify(key) {
	case ShouldNotPersist:
		return "", ErrPersistenceForbidden
	case MayStayPlain:
		return value, nil
	}
	if s.Cipher == nil {
		return "", ErrLocked
	}
	e, err := s.Cipher.Seal(key, []byte(value))
	if err != nil {
		return "", fmt.Errorf("seal protected record: %w", err)
	}
	raw, err := e.Encode()
	return string(raw), err
}

// Get reads and authenticates the durable record. Locked and recovery errors are
// preserved for errors.Is and must not be interpreted as an absent record.
func (s Storage) Get(key string) (string, bool, error) {
	value, present, err := s.Backend.Read(key)
	if err != nil || !present {
		return "", present, err
	}
	plain, err := s.Decode(key, value)
	return plain, true, err
}

// Set encrypts and conditionally replaces the value observed at the start of
// this operation. A concurrent writer causes an error rather than silent loss.
func (s Storage) Set(key, value string) error {
	stored, err := s.Encode(key, value)
	if err != nil {
		return err
	}
	old, present, err := s.Backend.Read(key)
	if err != nil {
		return err
	}
	return s.Backend.Replace(key, stored, expectedValue(old, present))
}

// Remove deletes a record only if it has not changed during the operation.
func (s Storage) Remove(key string) error {
	old, present, err := s.Backend.Read(key)
	if err != nil || !present {
		return err
	}
	return s.Backend.Delete(key, &old)
}

func expectedValue(value string, present bool) *string {
	if !present {
		return nil
	}
	return &value
}

// Migrate seals every plaintext sensitive value. Each temporary sealed copy is
// committed, read back and authenticated before replacing its original. Restart
// resumes from the temporary record; mismatched copies fail without overwriting.
// Transient secrets are deliberately left to the caller's memory-only migration.
// The returned keys are the records successfully migrated in this invocation.
func (s Storage) Migrate() ([]string, error) {
	keys, err := s.Backend.Keys()
	if err != nil {
		return nil, err
	}
	unique := map[string]bool{}
	for _, key := range keys {
		unique[strings.TrimPrefix(key, MigrationPrefix)] = true
	}
	keys = keys[:0]
	for key := range unique {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var migrated []string
	for _, key := range keys {
		if Classify(key) != MustEncrypt {
			continue
		}
		changed, err := s.migrateOne(key)
		if err != nil {
			return migrated, fmt.Errorf("migrate protected record %q: %w", key, err)
		}
		if changed {
			migrated = append(migrated, key)
		}
	}
	return migrated, nil
}

func (s Storage) migrateOne(key string) (bool, error) {
	old, present, err := s.Backend.Read(key)
	if err != nil {
		return false, err
	}
	stageKey := MigrationPrefix + key
	stage, staged, err := s.Backend.Read(stageKey)
	if err != nil {
		return false, err
	}
	if !present {
		// Never resurrect a deleted original from a stale migration record.
		if staged {
			return false, ErrRecordChanged
		}
		return false, nil
	}
	if IsSealed(old) {
		if _, err := s.Decode(key, old); err != nil {
			return false, err
		}
		if staged {
			a, err := s.Decode(key, old)
			if err != nil {
				return false, err
			}
			b, err := s.Decode(key, stage)
			if err != nil {
				return false, err
			}
			if a != b {
				return false, ErrRecordChanged
			}
			return false, s.Backend.Delete(stageKey, &stage)
		}
		return false, nil
	}
	if !staged {
		stage, err = s.Encode(key, old)
		if err != nil {
			return false, err
		}
		if err = s.Backend.Replace(stageKey, stage, nil); err != nil {
			return false, err
		}
	}
	// This is an actual backend read, not a cache of the bytes just submitted.
	verified, have, err := s.Backend.Read(stageKey)
	if err != nil {
		return false, err
	}
	if !have || verified != stage {
		return false, ErrRecordChanged
	}
	plain, err := s.Decode(key, verified)
	if err != nil {
		return false, err
	}
	if !bytes.Equal([]byte(plain), []byte(old)) {
		return false, ErrRecordChanged
	}
	if err := s.Backend.Replace(key, verified, &old); err != nil {
		return false, err
	}
	if _, _, err := s.Get(key); err != nil {
		return false, err
	}
	return true, s.Backend.Delete(stageKey, &stage)
}
