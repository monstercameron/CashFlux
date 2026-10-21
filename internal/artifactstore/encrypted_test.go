// SPDX-License-Identifier: MIT

package artifactstore

import (
	"bytes"
	"errors"
	"testing"
)

type memoryRawStore struct {
	mime string
	data []byte
	fail bool
}

func (m *memoryRawStore) Put(_ string, mime string, data []byte) error {
	if m.fail {
		return errors.New("durable write failed")
	}
	m.mime, m.data = mime, append([]byte(nil), data...)
	return nil
}
func (m *memoryRawStore) Get(string) (string, []byte, bool, error) {
	if m.mime == "" {
		return "", nil, false, nil
	}
	return m.mime, append([]byte(nil), m.data...), true, nil
}
func (m *memoryRawStore) Delete(string) error   { m.mime, m.data = "", nil; return nil }
func (m *memoryRawStore) Usage() (int64, error) { return int64(len(m.data)), nil }

type testArtifactCipher struct{}

func (testArtifactCipher) SealArtifact(_ string, plain []byte) ([]byte, error) {
	out := []byte("sealed:")
	for _, b := range plain {
		out = append(out, b^0xa5)
	}
	return out, nil
}
func (testArtifactCipher) OpenArtifact(_ string, sealed []byte) ([]byte, error) {
	if !bytes.HasPrefix(sealed, []byte("sealed:")) {
		return nil, errors.New("invalid envelope")
	}
	out := make([]byte, len(sealed)-7)
	for i, b := range sealed[7:] {
		out[i] = b ^ 0xa5
	}
	return out, nil
}

func TestEncryptedStoreHidesMIMEAndBytesAndMigratesLegacy(t *testing.T) {
	raw := &memoryRawStore{}
	store := EncryptedStore{Raw: raw, Cipher: testArtifactCipher{}}
	file := []byte("private statement contents")
	if err := store.Put("pdf-1", "application/pdf", file); err != nil {
		t.Fatal(err)
	}
	if raw.mime != EncryptedMIME || bytes.Contains(raw.data, file) || bytes.Contains(raw.data, []byte("application/pdf")) {
		t.Fatal("raw store exposed local artifact contents or MIME")
	}
	mime, got, ok, err := store.Get("pdf-1")
	if err != nil || !ok || mime != "application/pdf" || !bytes.Equal(got, file) {
		t.Fatalf("encrypted roundtrip = %q, %q, %v, %v", mime, got, ok, err)
	}
	raw.mime, raw.data = "application/pdf", append([]byte(nil), file...)
	mime, got, ok, err = store.Get("pdf-1")
	if err != nil || !ok || mime != "application/pdf" || !bytes.Equal(got, file) || raw.mime != EncryptedMIME || bytes.Contains(raw.data, file) {
		t.Fatalf("legacy migration = %q, %q, %v, %v", mime, got, ok, err)
	}
}

func TestEncryptedStoreRefusesFailedLegacyMigration(t *testing.T) {
	raw := &memoryRawStore{mime: "application/pdf", data: []byte("private"), fail: true}
	store := EncryptedStore{Raw: raw, Cipher: testArtifactCipher{}}
	if _, _, ok, err := store.Get("pdf-1"); err == nil || ok {
		t.Fatalf("failed durable migration returned a readable file: ok=%v err=%v", ok, err)
	}
	if raw.mime != "application/pdf" || !bytes.Equal(raw.data, []byte("private")) {
		t.Fatal("failed migration changed the legacy record")
	}
}
