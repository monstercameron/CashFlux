// SPDX-License-Identifier: MIT

package artifactstore

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// EncryptedMIME marks an authenticated local artifact record. The actual MIME
// and file bytes are sealed together; this marker discloses neither.
const EncryptedMIME = "application/x-cashflux-encrypted-artifact"

// Cipher seals artifact contents under a workspace-scoped key bound to the ID.
type Cipher interface {
	SealArtifact(id string, plaintext []byte) ([]byte, error)
	OpenArtifact(id string, sealed []byte) ([]byte, error)
}

// EncryptedStore keeps binary data and MIME private in the underlying store.
// Legacy plaintext entries are replaced with a durable encrypted record on
// first read. A failed migration fails the read; it never claims protection.
type EncryptedStore struct {
	Raw    Store
	Cipher Cipher
}

func (s EncryptedStore) Put(id, mime string, data []byte) error {
	if s.Raw == nil || s.Cipher == nil {
		return ErrUnavailable
	}
	if len(mime) > 65535 {
		return errors.New("artifactstore: MIME is too long")
	}
	plain := make([]byte, 2+len(mime)+len(data))
	binary.BigEndian.PutUint16(plain, uint16(len(mime)))
	copy(plain[2:], mime)
	copy(plain[2+len(mime):], data)
	sealed, err := s.Cipher.SealArtifact(id, plain)
	if err != nil {
		return fmt.Errorf("artifactstore: encrypt %s: %w", id, err)
	}
	return s.Raw.Put(id, EncryptedMIME, sealed)
}

func (s EncryptedStore) Get(id string) (mime string, data []byte, ok bool, err error) {
	if s.Raw == nil || s.Cipher == nil {
		return "", nil, false, ErrUnavailable
	}
	mime, data, ok, err = s.Raw.Get(id)
	if err != nil || !ok {
		return mime, data, ok, err
	}
	if mime != EncryptedMIME {
		if err := s.Put(id, mime, data); err != nil {
			return "", nil, false, fmt.Errorf("artifactstore: migrate legacy %s: %w", id, err)
		}
		return mime, data, true, nil
	}
	plain, err := s.Cipher.OpenArtifact(id, data)
	if err != nil {
		return "", nil, false, fmt.Errorf("artifactstore: decrypt %s: %w", id, err)
	}
	if len(plain) < 2 {
		return "", nil, false, errors.New("artifactstore: encrypted artifact is truncated")
	}
	n := int(binary.BigEndian.Uint16(plain))
	if len(plain) < 2+n {
		return "", nil, false, errors.New("artifactstore: encrypted MIME is truncated")
	}
	return string(plain[2 : 2+n]), plain[2+n:], true, nil
}

func (s EncryptedStore) Delete(id string) error {
	if s.Raw == nil {
		return ErrUnavailable
	}
	return s.Raw.Delete(id)
}

func (s EncryptedStore) Usage() (int64, error) {
	if s.Raw == nil {
		return 0, ErrUnavailable
	}
	return s.Raw.Usage()
}
