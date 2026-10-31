// SPDX-License-Identifier: MIT

package artifactstore

import (
	"bytes"
	"errors"
	"slices"
	"testing"
)

type migrationRecord struct {
	mime string
	data []byte
}

type migrationStore struct {
	records       map[string]migrationRecord
	beforeReplace func(id string)
	readErrorID   string
}

func (m *migrationStore) Put(id, mime string, data []byte) error {
	m.records[id] = migrationRecord{mime, bytes.Clone(data)}
	return nil
}
func (m *migrationStore) Get(id string) (string, []byte, bool, error) {
	if id == m.readErrorID {
		return "", nil, false, errors.New("read failed")
	}
	r, ok := m.records[id]
	return r.mime, bytes.Clone(r.data), ok, nil
}
func (m *migrationStore) Delete(id string) error { delete(m.records, id); return nil }
func (m *migrationStore) Usage() (int64, error)  { return 0, nil }
func (m *migrationStore) Keys() ([]string, error) {
	keys := make([]string, 0, len(m.records))
	for id := range m.records {
		keys = append(keys, id)
	}
	slices.Sort(keys)
	return keys, nil
}
func (m *migrationStore) ReplaceIfUnchanged(id, oldMIME string, oldData []byte, newMIME string, newData []byte) (bool, error) {
	if m.beforeReplace != nil {
		m.beforeReplace(id)
	}
	r, ok := m.records[id]
	if !ok || r.mime != oldMIME || !bytes.Equal(r.data, oldData) {
		return false, nil
	}
	return true, m.Put(id, newMIME, newData)
}

func TestMigrateAllLegacyProtectsOrphanWithoutRewritingEncryptedFiles(t *testing.T) {
	raw := &migrationStore{records: map[string]migrationRecord{
		"orphan": {mime: "image/png", data: []byte("private orphan bytes")},
	}}
	store := EncryptedStore{Raw: raw, Cipher: testArtifactCipher{}}
	if err := store.Put("current", "application/pdf", []byte("already protected")); err != nil {
		t.Fatal(err)
	}
	protected := bytes.Clone(raw.records["current"].data)
	count, err := store.MigrateAllLegacy()
	if err != nil || count != 1 {
		t.Fatalf("migration count=%d err=%v", count, err)
	}
	if raw.records["orphan"].mime != EncryptedMIME || bytes.Contains(raw.records["orphan"].data, []byte("private orphan")) {
		t.Fatal("orphan remains readable in raw storage")
	}
	if !bytes.Equal(raw.records["current"].data, protected) {
		t.Fatal("migration rewrote an encrypted record")
	}
	mime, data, ok, err := store.Get("orphan")
	if err != nil || !ok || mime != "image/png" || !bytes.Equal(data, []byte("private orphan bytes")) {
		t.Fatalf("orphan roundtrip mime=%q data=%q ok=%v err=%v", mime, data, ok, err)
	}
}

func TestMigrateAllLegacyDoesNotOverwriteConcurrentReplacement(t *testing.T) {
	raw := &migrationStore{records: map[string]migrationRecord{
		"file": {mime: "image/png", data: []byte("old file")},
	}}
	raw.beforeReplace = func(id string) {
		raw.beforeReplace = nil
		raw.records[id] = migrationRecord{mime: "image/png", data: []byte("new file")}
	}
	count, err := (EncryptedStore{Raw: raw, Cipher: testArtifactCipher{}}).MigrateAllLegacy()
	if err != nil || count != 0 {
		t.Fatalf("concurrent migration count=%d err=%v", count, err)
	}
	if !bytes.Equal(raw.records["file"].data, []byte("new file")) {
		t.Fatal("migration overwrote a concurrent file update")
	}
}

func TestMigrateAllLegacyContinuesAfterReadFailure(t *testing.T) {
	raw := &migrationStore{records: map[string]migrationRecord{
		"bad":  {mime: "image/png", data: []byte("unreadable")},
		"good": {mime: "image/png", data: []byte("readable")},
	}, readErrorID: "bad"}
	count, err := (EncryptedStore{Raw: raw, Cipher: testArtifactCipher{}}).MigrateAllLegacy()
	if count != 1 || err == nil || raw.records["good"].mime != EncryptedMIME {
		t.Fatalf("partial migration count=%d err=%v", count, err)
	}
}
