// SPDX-License-Identifier: MIT

package artifactstore

import (
	"bytes"
	"errors"
	"fmt"
)

// LegacyInventory lists raw local records and conditionally replaces a record
// only while its original MIME and bytes are still present. This keeps a
// background migration from overwriting an upload in another browser tab.
type LegacyInventory interface {
	Keys() ([]string, error)
	ReplaceIfUnchanged(id, oldMIME string, oldData []byte, newMIME string, newData []byte) (bool, error)
}

// MigrateAllLegacy seals every plaintext record in the raw store, including
// records no saved workspace currently references. A concurrent replacement is
// left alone; a later boot or read can visit it again. Errors are joined so one
// damaged file does not prevent other files from being protected.
func (s EncryptedStore) MigrateAllLegacy() (int, error) {
	if s.Raw == nil || s.Cipher == nil {
		return 0, ErrUnavailable
	}
	inventory, ok := s.Raw.(LegacyInventory)
	if !ok {
		return 0, ErrUnavailable
	}
	keys, err := inventory.Keys()
	if err != nil {
		return 0, fmt.Errorf("artifactstore: list local records: %w", err)
	}
	var problems []error
	migrated := 0
	for _, id := range keys {
		mime, data, present, err := s.Raw.Get(id)
		if err != nil {
			problems = append(problems, fmt.Errorf("artifactstore: read %s: %w", id, err))
			continue
		}
		if !present || mime == EncryptedMIME {
			continue
		}
		if bytes.IndexByte([]byte(mime), 0) >= 0 {
			problems = append(problems, fmt.Errorf("artifactstore: MIME for %s contains a zero byte", id))
			continue
		}
		plain := make([]byte, len(mime)+1+len(data))
		copy(plain, mime)
		plain[len(mime)] = 0
		copy(plain[len(mime)+1:], data)
		sealed, err := s.Cipher.SealArtifact(id, plain)
		if err != nil {
			problems = append(problems, fmt.Errorf("artifactstore: encrypt %s: %w", id, err))
			continue
		}
		replaced, err := inventory.ReplaceIfUnchanged(id, mime, data, EncryptedMIME, sealed)
		if err != nil {
			problems = append(problems, fmt.Errorf("artifactstore: replace %s: %w", id, err))
			continue
		}
		if replaced {
			migrated++
		}
	}
	return migrated, errors.Join(problems...)
}
