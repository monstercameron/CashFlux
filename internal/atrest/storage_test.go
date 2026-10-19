package atrest

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/keyring"
)

type recordMemory struct {
	values       map[string]string
	operations   int
	failAt       int
	afterReplace func(string)
}

func (m *recordMemory) Read(key string) (string, bool, error) {
	v, ok := m.values[key]
	return v, ok, nil
}
func (m *recordMemory) Keys() ([]string, error) {
	var keys []string
	for k := range m.values {
		keys = append(keys, k)
	}
	return keys, nil
}
func (m *recordMemory) check(key string, expected *string) error {
	m.operations++
	if m.operations == m.failAt {
		return errors.New("injected commit failure")
	}
	v, have := m.values[key]
	if expected == nil && have || expected != nil && (!have || v != *expected) {
		return ErrRecordChanged
	}
	return nil
}
func (m *recordMemory) Replace(key, value string, expected *string) error {
	if err := m.check(key, expected); err != nil {
		return err
	}
	m.values[key] = value
	if m.afterReplace != nil {
		m.afterReplace(key)
	}
	return nil
}
func (m *recordMemory) Delete(key string, expected *string) error {
	if err := m.check(key, expected); err != nil {
		return err
	}
	delete(m.values, key)
	return nil
}

type recordTestCipher struct {
	key       *keyring.WorkspaceKey
	scope     keyring.Scope
	workspace string
	locked    bool
}

func (c *recordTestCipher) Seal(key string, plain []byte) (Envelope, error) {
	if c.locked {
		return Envelope{}, ErrLocked
	}
	return SealRecord(c.key, c.scope, c.workspace, key, plain)
}
func (c *recordTestCipher) Open(key string, e Envelope) ([]byte, error) {
	if c.locked {
		return nil, ErrLocked
	}
	return OpenRecord(c.key, c.scope, c.workspace, key, e)
}
func testStorage(t *testing.T) (Storage, *recordMemory, *recordTestCipher) {
	t.Helper()
	wk, err := keyring.NewWorkspaceKey()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(wk.Destroy)
	m := &recordMemory{values: map[string]string{}}
	c := &recordTestCipher{key: wk, scope: keyring.LocalScope("household"), workspace: "workspace"}
	return Storage{Backend: m, Cipher: c}, m, c
}

func TestStorageCatalogueBoundary(t *testing.T) {
	for _, tc := range []struct {
		key            string
		plain, refused bool
	}{
		{"cashflux:dataset", false, false}, {"cashflux:theme", true, false},
		{"cashflux:unknown-sensitive-key", false, false}, {"cashflux:auth:access-token", false, true},
		{"cashflux:auth:refresh-token", false, true}, {"cashflux:passcode", false, true},
	} {
		t.Run(tc.key, func(t *testing.T) {
			s, m, _ := testStorage(t)
			const payload = "Blue Bottle 1,850 Checking"
			err := s.Set(tc.key, payload)
			if tc.refused {
				if !errors.Is(err, ErrPersistenceForbidden) || len(m.values) != 0 {
					t.Fatalf("refused write: %v, %v", err, m.values)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(m.values[tc.key], payload) != tc.plain {
				t.Fatal("wrong at-rest classification")
			}
			if got, have, err := s.Get(tc.key); err != nil || !have || got != payload {
				t.Fatalf("roundtrip %q %v %v", got, have, err)
			}
		})
	}
}

func TestStorageLockedAndWrongRecordAreErrors(t *testing.T) {
	s, m, c := testStorage(t)
	if err := s.Set("cashflux:dataset", "private"); err != nil {
		t.Fatal(err)
	}
	c.locked = true
	if _, have, err := s.Get("cashflux:dataset"); !have || !errors.Is(err, ErrLocked) {
		t.Fatalf("locked read: %v %v", have, err)
	}
	if err := s.Set("cashflux:dataset", "replacement"); !errors.Is(err, ErrLocked) {
		t.Fatal(err)
	}
	c.locked = false
	m.values["cashflux:sync-queue"] = m.values["cashflux:dataset"]
	if _, _, err := s.Get("cashflux:sync-queue"); err == nil {
		t.Fatal("copied ciphertext authenticated with different key")
	}
}

func TestMigrationResumesAfterEveryCommitFailure(t *testing.T) {
	for _, failAt := range []int{1, 2, 3} {
		t.Run(string(rune('0'+failAt)), func(t *testing.T) {
			s, m, _ := testStorage(t)
			const key = "cashflux:dataset"
			const original = "not even JSON: Blue Bottle \x00 1,850"
			m.values[key] = original
			m.failAt = failAt
			if _, err := s.Migrate(); err == nil {
				t.Fatal("commit failure acknowledged")
			}
			if failAt <= 2 && m.values[key] != original {
				t.Fatal("source removed before sealed copy committed and verified")
			}
			m.failAt = 0
			if _, err := s.Migrate(); err != nil {
				t.Fatal(err)
			}
			if got, _, err := s.Get(key); err != nil || got != original {
				t.Fatalf("lost bytes: %q %v", got, err)
			}
			if len(m.values) != 1 {
				t.Fatalf("staging not cleaned up: %v", m.values)
			}
			before := m.values[key]
			if keys, err := s.Migrate(); err != nil || len(keys) != 0 || before != m.values[key] {
				t.Fatal("migration is not idempotent", keys, err)
			}
		})
	}
}

func TestMigrationReadbackAndConcurrentChangePreserveOriginal(t *testing.T) {
	for _, mode := range []string{"corrupt-stage", "change-original", "change-stage", "delete-original"} {
		t.Run(mode, func(t *testing.T) {
			s, m, _ := testStorage(t)
			const key = "cashflux:dataset"
			m.values[key] = "original"
			m.afterReplace = func(k string) {
				if k != MigrationPrefix+key {
					return
				}
				switch mode {
				case "corrupt-stage":
					m.values[k] = `{"format":"cashflux-atrest"}`
				case "change-original":
					m.values[key] = "newer value"
				case "change-stage":
					e, err := s.Encode(key, "other value")
					if err != nil {
						t.Fatal(err)
					}
					m.values[k] = e
				case "delete-original":
					delete(m.values, key)
				}
			}
			if _, err := s.Migrate(); err == nil {
				t.Fatal("changed data silently replaced")
			}
			want := "original"
			if mode == "change-original" {
				want = "newer value"
			}
			if mode == "delete-original" {
				want = ""
			}
			if m.values[key] != want {
				t.Fatalf("source changed: %q", m.values[key])
			}
		})
	}
}

func TestMigrationReportsAllSensitiveKeysAndPreservesChrome(t *testing.T) {
	s, m, _ := testStorage(t)
	want := []string{"cashflux:checkpoint-data:1", "cashflux:dataset", "cashflux:member-pins", "cashflux:notify:feed", "cashflux:quarantine:1", "cashflux:sync-queue", "cashflux:tx-filter", "cashflux:ws-data:other"}
	for _, k := range want {
		m.values[k] = "Blue Bottle"
	}
	m.values["cashflux:theme"] = "dark"
	got, err := s.Migrate()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("migrated %v want %v", got, want)
	}
	for _, k := range want {
		if strings.Contains(m.values[k], "Blue Bottle") {
			t.Fatalf("plaintext %s", k)
		}
	}
	if m.values["cashflux:theme"] != "dark" {
		t.Fatal("chrome changed")
	}
}

func TestMalformedEnvelopeNeverBecomesLegacyPlaintext(t *testing.T) {
	s, m, _ := testStorage(t)
	for _, raw := range []string{`{"format":"cashflux-atrest"}`, `{"format":"cashflux-atrest",`, `{"format":"cashflux-atrest","version":999}`} {
		m.values["cashflux:dataset"] = raw
		if _, err := s.Migrate(); err == nil {
			t.Fatal("malformed envelope resealed")
		}
		if m.values["cashflux:dataset"] != raw {
			t.Fatal("malformed envelope overwritten")
		}
	}
}

func TestEnvelopeDetectionDoesNotMistakeHouseholdTextForAnEnvelope(t *testing.T) {
	for _, value := range []string{`{"notes":"cashflux-atrest"}`, `"cashflux-atrest"`, `{"nested":{"format":"cashflux-atrest"}}`} {
		if IsSealed(value) {
			t.Fatalf("household payload misclassified: %s", value)
		}
	}
	if !IsSealed(`{"format":"cashflux-atrest-v2","version":2}`) {
		t.Fatal("future envelope mistaken for plaintext")
	}
}
