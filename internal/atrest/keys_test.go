package atrest

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/monstercameron/CashFlux/internal/keyring"
)

type memoryKeyStore struct {
	mu               sync.Mutex
	state            *KeyState
	device           keyring.Wrapper
	fail             error
	loadErr          error
	creates, commits int
}

func (m *memoryKeyStore) Load() (*KeyState, keyring.Wrapper, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state.Clone(), m.device, m.loadErr
}

func (m *memoryKeyStore) NewDevice() (keyring.Wrapper, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.creates++
	key := make([]byte, keyring.KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return keyring.AESGCMWrapper{Key: key}, nil
}

func (m *memoryKeyStore) Commit(expected uint64, state *KeyState, device keyring.Wrapper) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail != nil {
		return m.fail
	}
	var revision uint64
	if m.state != nil {
		revision = m.state.Revision
	}
	if revision != expected {
		return ErrKeyStateChanged
	}
	m.state, m.device = state.Clone(), device
	m.commits++
	return nil
}

// fastDeriver exercises wrap authentication deterministically; production
// PBKDF2 parameters and WebCrypto compatibility are verified in the wasm tests.
var fastDeriver = keyring.PBKDF2Func(func(password string, salt []byte, _, _ int) ([]byte, error) {
	h := sha256.New()
	h.Write([]byte(password))
	h.Write(salt)
	return h.Sum(nil), nil
})

var keysScope = keyring.Scope{ServerID: "server-a", AccountID: "account-a"}

func openMemoryKeys(t *testing.T, store *memoryKeyStore) *KeyService {
	t.Helper()
	s, err := OpenKeyService(store, keysScope, "workspace-a", store.state != nil)
	if err != nil {
		t.Fatal(err)
	}
	s.deriver = fastDeriver
	t.Cleanup(s.Lock)
	return s
}

func keyStateBytes(t *testing.T, state *KeyState) []byte {
	t.Helper()
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func assertRecord(t *testing.T, service *KeyService, envelope Envelope) {
	t.Helper()
	plain, err := service.Open("cashflux:dataset", envelope)
	if err != nil || string(plain) != "household secret" {
		t.Fatalf("open = %q, %v", plain, err)
	}
}

func TestKeyServiceLifecycle(t *testing.T) {
	store := &memoryKeyStore{}
	s := openMemoryKeys(t, store)
	if store.creates != 1 || store.commits != 1 || s.State().AppLock {
		t.Fatal("first use did not commit one device and keyring")
	}
	envelope, err := s.Seal("cashflux:dataset", []byte("household secret"))
	if err != nil {
		t.Fatal(err)
	}
	keyID := envelope.KeyID
	s.Lock()
	reloaded := openMemoryKeys(t, store)
	assertRecord(t, reloaded, envelope)
	if store.creates != 1 {
		t.Fatal("reload generated a replacement key")
	}
	code, err := reloaded.AddRecovery()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(keyStateBytes(t, store.state), []byte(code)) {
		t.Fatal("recovery code was persisted")
	}
	if err := reloaded.EnableAppLock("first passcode"); err != nil {
		t.Fatal(err)
	}
	assertRecord(t, reloaded, envelope)
	for _, wrap := range store.state.Ring.Current().Wraps {
		if wrap.Kind == keyring.WrapDevice {
			t.Fatal("App Lock retained a device-only wrap")
		}
	}
	locked := openMemoryKeys(t, store)
	for _, test := range []struct {
		name string
		call func() error
	}{
		{"seal", func() error { _, err := locked.Seal("cashflux:dataset", nil); return err }},
		{"open", func() error { _, err := locked.Open("cashflux:dataset", envelope); return err }},
		{"change passcode", func() error { return locked.ChangePasscode("next") }},
		{"disable lock", locked.DisableAppLock},
		{"add recovery", func() error { _, err := locked.AddRecovery(); return err }},
		{"repair device", locked.RepairDevice},
	} {
		t.Run("locked/"+test.name, func(t *testing.T) {
			if err := test.call(); !errors.Is(err, ErrLocked) {
				t.Fatalf("got %v, want ErrLocked", err)
			}
		})
	}
	before := keyStateBytes(t, store.state)
	if err := locked.UnlockPasscode("wrong"); !errors.Is(err, keyring.ErrUnwrapFailed) {
		t.Fatalf("wrong passcode: %v", err)
	}
	if !bytes.Equal(before, keyStateBytes(t, store.state)) {
		t.Fatal("wrong passcode changed storage")
	}
	if err := locked.UnlockPasscode("first passcode"); err != nil {
		t.Fatal(err)
	}
	assertRecord(t, locked, envelope)
	if err := locked.ChangePasscode("second passcode"); err != nil {
		t.Fatal(err)
	}
	if locked.State().Ring.Current().KeyID != keyID {
		t.Fatal("passcode change rotated data key")
	}
	locked.Lock()
	if err := locked.UnlockPasscode("first passcode"); !errors.Is(err, keyring.ErrUnwrapFailed) {
		t.Fatalf("old passcode accepted: %v", err)
	}
	if err := locked.UnlockPasscode("second passcode"); err != nil {
		t.Fatal(err)
	}
	assertRecord(t, locked, envelope)
	locked.Lock()
	if err := locked.UnlockRecovery(code); err != nil {
		t.Fatal(err)
	}
	assertRecord(t, locked, envelope)
	if err := locked.DisableAppLock(); err != nil {
		t.Fatal(err)
	}
	if locked.State().AppLock {
		t.Fatal("App Lock still enabled")
	}
	for _, wrap := range store.state.Ring.Current().Wraps {
		if wrap.Kind == keyring.WrapPasscode {
			t.Fatal("disabled App Lock retained passcode wrap")
		}
	}
	assertRecord(t, openMemoryKeys(t, store), envelope)
}

func TestKeyServiceMissingKeyNeverResets(t *testing.T) {
	for _, test := range []struct {
		name   string
		locked bool
	}{{"device only", false}, {"app lock", true}} {
		t.Run(test.name, func(t *testing.T) {
			store := &memoryKeyStore{}
			s := openMemoryKeys(t, store)
			envelope, err := s.Seal("cashflux:dataset", []byte("household secret"))
			if err != nil {
				t.Fatal(err)
			}
			code, err := s.AddRecovery()
			if err != nil {
				t.Fatal(err)
			}
			if test.locked {
				if err := s.EnableAppLock("passcode"); err != nil {
					t.Fatal(err)
				}
			}
			s.Lock()
			store.device = nil
			before := keyStateBytes(t, store.state)
			creates, commits := store.creates, store.commits
			recovery, err := OpenKeyService(store, keysScope, "workspace-a", true)
			if recovery == nil || !errors.Is(err, ErrRecoveryRequired) || !errors.Is(err, ErrMissingDeviceKey) {
				t.Fatalf("missing device = %v, %v", recovery, err)
			}
			t.Cleanup(recovery.Lock)
			if store.creates != creates || store.commits != commits || !bytes.Equal(before, keyStateBytes(t, store.state)) {
				t.Fatal("missing device changed storage")
			}
			if err := recovery.UnlockRecovery("invalid"); !errors.Is(err, keyring.ErrInvalidRecoveryCode) {
				t.Fatalf("invalid recovery code: %v", err)
			}
			if err := recovery.UnlockRecovery(code); err != nil {
				t.Fatal(err)
			}
			assertRecord(t, recovery, envelope)
			if err := recovery.RepairDevice(); err != nil {
				t.Fatal(err)
			}
			reloaded := openMemoryKeys(t, store)
			if test.locked {
				for _, wrap := range reloaded.State().Ring.Current().Wraps {
					if wrap.Kind == keyring.WrapDevice {
						t.Fatal("repair bypasses lock")
					}
				}
				if err := reloaded.UnlockRecovery(code); err != nil {
					t.Fatal(err)
				}
			}
			assertRecord(t, reloaded, envelope)
		})
	}
	t.Run("missing keyring with ciphertext", func(t *testing.T) {
		store := &memoryKeyStore{}
		s, err := OpenKeyService(store, keysScope, "workspace-a", true)
		if s != nil || !errors.Is(err, ErrRecoveryRequired) || store.creates != 0 || store.commits != 0 {
			t.Fatalf("silently created keys: %v, %v", s, err)
		}
	})
}

func TestKeyServiceFailuresPreserveState(t *testing.T) {
	for _, test := range []struct {
		name   string
		locked bool
		call   func(*KeyService) error
	}{
		{"enable commit", false, func(s *KeyService) error { return s.EnableAppLock("passcode") }},
		{"disable commit", true, (*KeyService).DisableAppLock},
		{"rekey commit", true, func(s *KeyService) error { return s.ChangePasscode("new") }},
		{"recovery commit", false, func(s *KeyService) error {
			code, err := s.AddRecovery()
			if err != nil && code != "" {
				t.Error("returned uncommitted recovery code")
			}
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &memoryKeyStore{}
			s := openMemoryKeys(t, store)
			if test.locked {
				if err := s.EnableAppLock("passcode"); err != nil {
					t.Fatal(err)
				}
			}
			before := keyStateBytes(t, store.state)
			localBefore := keyStateBytes(t, s.State())
			failure := errors.New("quota exhausted")
			store.fail = failure
			if err := test.call(s); !errors.Is(err, failure) {
				t.Fatalf("failure = %v", err)
			}
			if !bytes.Equal(before, keyStateBytes(t, store.state)) || !bytes.Equal(localBefore, keyStateBytes(t, s.State())) {
				t.Fatal("failed write published key state")
			}
			store.fail = nil
			if err := test.call(s); err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Run("first use commit", func(t *testing.T) {
		store := &memoryKeyStore{fail: errors.New("storage full")}
		if s, err := OpenKeyService(store, keysScope, "workspace-a", false); s != nil || err == nil || store.state != nil {
			t.Fatalf("first use acknowledged failure: %v, %v", s, err)
		}
	})
}

func TestKeyServiceTamperAndScope(t *testing.T) {
	store := &memoryKeyStore{}
	s := openMemoryKeys(t, store)
	envelope, err := s.Seal("cashflux:dataset", []byte("household secret"))
	if err != nil {
		t.Fatal(err)
	}
	before := keyStateBytes(t, store.state)
	for _, test := range []struct {
		name   string
		change func(*Envelope)
		record string
		want   error
	}{
		{"ciphertext", func(e *Envelope) { e.Ciphertext[0] ^= 1 }, "cashflux:dataset", keyring.ErrAuthenticationFailed},
		{"nonce", func(e *Envelope) { e.Nonce[0] ^= 1 }, "cashflux:dataset", keyring.ErrAuthenticationFailed},
		{"key id", func(e *Envelope) { e.KeyID[0] ^= 1 }, "cashflux:dataset", ErrKeyMismatch},
		{"record key", func(*Envelope) {}, "cashflux:prefs", keyring.ErrAuthenticationFailed},
	} {
		t.Run(test.name, func(t *testing.T) {
			copyEnvelope := envelope
			copyEnvelope.Ciphertext = bytes.Clone(envelope.Ciphertext)
			copyEnvelope.Nonce = bytes.Clone(envelope.Nonce)
			test.change(&copyEnvelope)
			if _, err := s.Open(test.record, copyEnvelope); !errors.Is(err, test.want) {
				t.Fatalf("tamper = %v, want %v", err, test.want)
			}
			if !bytes.Equal(before, keyStateBytes(t, store.state)) {
				t.Fatal("tamper changed key state")
			}
		})
	}
	for _, test := range []struct {
		name      string
		scope     keyring.Scope
		workspace string
	}{
		{"server", keyring.Scope{ServerID: "server-b", AccountID: keysScope.AccountID}, "workspace-a"},
		{"account", keyring.Scope{ServerID: keysScope.ServerID, AccountID: "account-b"}, "workspace-a"},
		{"workspace", keysScope, "workspace-b"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := OpenKeyService(store, test.scope, test.workspace, true); !errors.Is(err, ErrInvalidKeyState) {
				t.Fatalf("foreign state: %v", err)
			}
		})
	}
	store.state.Ring.Current().Wraps[0].Wrapped[0] ^= 1
	tampered := keyStateBytes(t, store.state)
	if _, err := OpenKeyService(store, keysScope, "workspace-a", true); !errors.Is(err, ErrRecoveryRequired) || !errors.Is(err, keyring.ErrAuthenticationFailed) {
		t.Fatalf("tampered device wrap: %v", err)
	}
	if !bytes.Equal(tampered, keyStateBytes(t, store.state)) {
		t.Fatal("tampered wrap overwritten")
	}
}

func TestKeyServiceStaleInstanceAndValidation(t *testing.T) {
	store := &memoryKeyStore{}
	a := openMemoryKeys(t, store)
	b := openMemoryKeys(t, store)
	if err := a.EnableAppLock("passcode"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Seal("record", []byte("secret")); !errors.Is(err, ErrKeyStateChanged) || b.key != nil {
		t.Fatalf("stale tab used WDK: %v", err)
	}
	if err := b.EnableAppLock("other"); !errors.Is(err, ErrKeyStateChanged) {
		t.Fatalf("stale change: %v", err)
	}
	for _, test := range []struct {
		name   string
		change func(*KeyState)
	}{
		{"zero revision", func(s *KeyState) { s.Revision = 0 }},
		{"lock flag", func(s *KeyState) { s.AppLock = false }},
		{"missing ring", func(s *KeyState) { s.Ring = nil }},
		{"empty wraps", func(s *KeyState) { s.Ring.Current().Wraps = nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			copyStore := &memoryKeyStore{state: store.state.Clone(), device: store.device}
			test.change(copyStore.state)
			if _, err := OpenKeyService(copyStore, keysScope, "workspace-a", true); !errors.Is(err, ErrInvalidKeyState) {
				t.Fatalf("invalid state: %v", err)
			}
			if copyStore.creates != 0 || copyStore.commits != 0 {
				t.Fatal("invalid state was rewritten")
			}
		})
	}
	t.Run("load failure", func(t *testing.T) {
		failure := fmt.Errorf("storage unavailable")
		store.loadErr = failure
		if _, err := a.Seal("record", nil); !errors.Is(err, failure) {
			t.Fatalf("load failure: %v", err)
		}
	})
}

func TestKeyServiceRechecksUnversionedTamper(t *testing.T) {
	store := &memoryKeyStore{}
	s := openMemoryKeys(t, store)
	store.state.Ring.Current().Wraps[0].Wrapped[0] ^= 1
	before := keyStateBytes(t, store.state)
	if _, err := s.Seal("record", []byte("secret")); !errors.Is(err, ErrKeyStateChanged) || s.key != nil {
		t.Fatalf("cached WDK used after metadata tamper: %v", err)
	}
	if !bytes.Equal(before, keyStateBytes(t, store.state)) {
		t.Fatal("unversioned tamper changed durable state")
	}
}

func TestKeyServiceRejectsInvalidTransitions(t *testing.T) {
	for _, test := range []struct {
		name   string
		locked bool
		call   func(*KeyService) error
		want   error
	}{
		{"disable already off", false, (*KeyService).DisableAppLock, ErrInvalidKeyState},
		{"change without lock", false, func(s *KeyService) error { return s.ChangePasscode("new") }, ErrInvalidKeyState},
		{"enable empty passcode", false, func(s *KeyService) error { return s.EnableAppLock("") }, keyring.ErrInvalidSecret},
		{"enable already on", true, func(s *KeyService) error { return s.EnableAppLock("new") }, ErrInvalidKeyState},
		{"change empty passcode", true, func(s *KeyService) error { return s.ChangePasscode("") }, keyring.ErrInvalidSecret},
		{"repair existing device", false, (*KeyService).RepairDevice, ErrInvalidKeyState},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &memoryKeyStore{}
			s := openMemoryKeys(t, store)
			if test.locked {
				if err := s.EnableAppLock("passcode"); err != nil {
					t.Fatal(err)
				}
			}
			before := keyStateBytes(t, store.state)
			if err := test.call(s); !errors.Is(err, test.want) {
				t.Fatalf("got %v, want %v", err, test.want)
			}
			if !bytes.Equal(before, keyStateBytes(t, store.state)) {
				t.Fatal("invalid transition changed key state")
			}
		})
	}
}

func TestKeyServiceTamperedPasscodeWrapPreserved(t *testing.T) {
	store := &memoryKeyStore{}
	s := openMemoryKeys(t, store)
	if err := s.EnableAppLock("passcode"); err != nil {
		t.Fatal(err)
	}
	s.Lock()
	store.state.Ring.Current().Wraps[0].Wrapped[0] ^= 1
	before := keyStateBytes(t, store.state)
	locked := openMemoryKeys(t, store)
	if err := locked.UnlockPasscode("passcode"); !errors.Is(err, keyring.ErrAuthenticationFailed) || !errors.Is(err, keyring.ErrUnwrapFailed) {
		t.Fatalf("tampered passcode wrap: %v", err)
	}
	if !bytes.Equal(before, keyStateBytes(t, store.state)) || locked.key != nil {
		t.Fatal("tampered passcode wrap changed data or unlocked")
	}
}

func TestKeyServiceMissingDevicePasscodeRepairFailure(t *testing.T) {
	store := &memoryKeyStore{}
	s := openMemoryKeys(t, store)
	envelope, err := s.Seal("cashflux:dataset", []byte("household secret"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.EnableAppLock("passcode"); err != nil {
		t.Fatal(err)
	}
	s.Lock()
	store.device = nil
	recovery, err := OpenKeyService(store, keysScope, "workspace-a", true)
	if !errors.Is(err, ErrMissingDeviceKey) || recovery == nil {
		t.Fatalf("missing device: %v", err)
	}
	t.Cleanup(recovery.Lock)
	recovery.deriver = fastDeriver
	if err := recovery.UnlockPasscode("passcode"); err != nil {
		t.Fatal(err)
	}
	assertRecord(t, recovery, envelope)
	if err := recovery.DisableAppLock(); !errors.Is(err, ErrMissingDeviceKey) {
		t.Fatalf("disable without device: %v", err)
	}
	before := keyStateBytes(t, store.state)
	failure := errors.New("repair commit failed")
	store.fail = failure
	if err := recovery.RepairDevice(); !errors.Is(err, failure) {
		t.Fatalf("repair failure: %v", err)
	}
	if store.device != nil || recovery.device != nil || !bytes.Equal(before, keyStateBytes(t, store.state)) {
		t.Fatal("failed repair published device or state")
	}
	store.fail = nil
	if err := recovery.RepairDevice(); err != nil {
		t.Fatal(err)
	}
	if err := recovery.DisableAppLock(); err != nil {
		t.Fatal(err)
	}
	assertRecord(t, openMemoryKeys(t, store), envelope)
}
