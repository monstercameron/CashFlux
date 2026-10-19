package atrest

import (
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/monstercameron/CashFlux/internal/keyring"
)

var (
	// ErrLocked means the workspace key must be unlocked before use.
	ErrLocked = errors.New("atrest: workspace is locked")
	// ErrMissingDeviceKey means the stored device CryptoKey is unavailable.
	ErrMissingDeviceKey = errors.New("atrest: device key is missing")
	// ErrRecoveryRequired means existing data must be recovered, never reset.
	ErrRecoveryRequired = errors.New("atrest: key recovery is required")
	// ErrKeyStateChanged means another instance committed a key-state change.
	// Reopen the service; this instance has discarded its in-memory WDK.
	ErrKeyStateChanged = errors.New("atrest: key state changed; reopen the workspace")
	// ErrInvalidKeyState means persisted key metadata is inconsistent.
	ErrInvalidKeyState = errors.New("atrest: invalid key state")
)

// KeyState contains only public metadata and wrapped key material. It is scoped
// to exactly one workspace. Revision is a monotonic CAS token, never a time.
type KeyState struct {
	// Revision starts at one and advances with every durable transition.
	Revision uint64 `json:"revision"`
	// Scope identifies the server and account.
	Scope keyring.Scope `json:"scope"`
	// WorkspaceID identifies the workspace within Scope.
	WorkspaceID string `json:"workspaceId"`
	// AppLock requires a passcode or recovery secret to unlock the WDK.
	AppLock bool `json:"appLock"`
	// Ring contains the wrapped workspace key, never its raw bytes.
	Ring *keyring.Keyring `json:"ring"`
}

// Clone returns an independent copy of the persisted state.
func (s *KeyState) Clone() *KeyState {
	if s == nil {
		return nil
	}
	out := *s
	out.Ring = s.Ring.Clone()
	return &out
}

// KeyStore is the durable boundary for key metadata and the opaque device key.
// Load returns independent state and a nil wrapper when the device key is absent.
// NewDevice generates an unpersisted wrapper. Commit atomically stores state and
// device (which may be nil), only if the stored revision equals expected (zero
// means absent). It returns only after strict-durability commit. Implementations
// must not serialize device key bytes; the browser stores a CryptoKey directly.
type KeyStore interface {
	Load() (*KeyState, keyring.Wrapper, error)
	NewDevice() (keyring.Wrapper, error)
	Commit(expected uint64, state *KeyState, device keyring.Wrapper) error
}

// KeyService owns one workspace WDK in memory. Browser callers must invoke its
// blocking operations from a goroutine, never directly inside a JS callback.
// No operation deletes records or silently replaces unreadable key material.
type KeyService struct {
	mu      sync.Mutex
	store   KeyStore
	state   *KeyState
	device  keyring.Wrapper
	key     *keyring.WorkspaceKey
	deriver keyring.PBKDF2Deriver
}

// OpenKeyService loads a workspace's keys, creating them only when both key
// metadata and ciphertext are absent. existingCiphertext MUST reflect the
// caller's durable encrypted-record inventory, including encrypted migration or
// quarantine copies that need this keyring. Legacy plaintext alone does not set
// this flag. A missing keyring with existing ciphertext returns
// ErrRecoveryRequired without writing.
// A locked workspace returns a usable, locked service and nil error. A missing
// device key returns the service and ErrRecoveryRequired; UnlockRecovery (or
// UnlockPasscode for a locked workspace) can still open its existing keyring.
func OpenKeyService(store KeyStore, scope keyring.Scope, workspaceID string, existingCiphertext bool) (*KeyService, error) {
	if store == nil || scope.ServerID == "" || (!scope.IsLocal() && scope.AccountID == "") || workspaceID == "" {
		return nil, ErrInvalidKeyState
	}
	state, device, err := store.Load()
	if err != nil {
		return nil, fmt.Errorf("atrest: load keys: %w", err)
	}
	s := &KeyService{store: store, device: device, deriver: platformDeriver()}
	if state == nil {
		if existingCiphertext || device != nil {
			return nil, ErrRecoveryRequired
		}
		wk, err := keyring.NewWorkspaceKey()
		if err != nil {
			return nil, err
		}
		defer func() {
			if s.key == nil {
				wk.Destroy()
			}
		}()
		device, err = store.NewDevice()
		if err != nil {
			return nil, fmt.Errorf("atrest: create device key: %w", err)
		}
		ring, err := keyring.NewKeyring(wk)
		if err != nil {
			return nil, err
		}
		if err := ring.AddDeviceWrap(wk, "", device); err != nil {
			return nil, err
		}
		state = &KeyState{Revision: 1, Scope: scope, WorkspaceID: workspaceID, Ring: ring}
		if err := store.Commit(0, state, device); err != nil {
			return nil, fmt.Errorf("atrest: create keys: %w", err)
		}
		s.state, s.device, s.key = state, device, wk
		return s, nil
	}
	if err := validateKeyState(state, scope, workspaceID); err != nil {
		return nil, err
	}
	s.state = state.Clone()
	if device == nil {
		return s, errors.Join(ErrRecoveryRequired, ErrMissingDeviceKey)
	}
	if state.AppLock {
		return s, nil
	}
	s.key, err = state.Ring.Unwrap(keyring.WrapDevice, "", keyring.UnwrapSecret{Device: device})
	if err != nil {
		return s, fmt.Errorf("%w: open device wrap: %w", ErrRecoveryRequired, err)
	}
	return s, nil
}

func validateKeyState(state *KeyState, scope keyring.Scope, workspaceID string) error {
	if state == nil || state.Revision == 0 || state.Scope != scope || state.WorkspaceID != workspaceID || state.Ring == nil {
		return ErrInvalidKeyState
	}
	if err := state.Ring.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidKeyState, err)
	}
	// Rotation is deliberately not exposed by this service yet. Reject unknown
	// histories instead of rewrapping only the newest key and orphaning old data.
	if len(state.Ring.Keys) != 1 {
		return fmt.Errorf("%w: multiple key versions require a rotation-aware service", ErrInvalidKeyState)
	}
	var device, passcode bool
	for _, w := range state.Ring.Current().Wraps {
		switch w.Kind {
		case keyring.WrapDevice:
			if w.ID != "" {
				return ErrInvalidKeyState
			}
			device = true
		case keyring.WrapPasscode:
			if w.ID != "" {
				return ErrInvalidKeyState
			}
			passcode = true
		}
	}
	if state.AppLock && (device || !passcode) || !state.AppLock && (!device || passcode) {
		return ErrInvalidKeyState
	}
	return nil
}

// State returns a copy of the last observed metadata, containing no secrets.
func (s *KeyService) State() *KeyState { s.mu.Lock(); defer s.mu.Unlock(); return s.state.Clone() }

// Lock destroys the in-memory WDK. It never changes durable wraps.
func (s *KeyService) Lock() { s.mu.Lock(); defer s.mu.Unlock(); s.destroy() }

func (s *KeyService) destroy() {
	if s.key != nil {
		s.key.Destroy()
		s.key = nil
	}
}

// check rejects stale instances before using cached keys. Even a tab that was
// unlocked before another tab enabled App Lock must reopen after that change.
func (s *KeyService) check() error {
	state, _, err := s.store.Load()
	if err != nil {
		return fmt.Errorf("atrest: check keys: %w", err)
	}
	if state == nil || !reflect.DeepEqual(state, s.state) {
		s.destroy()
		return ErrKeyStateChanged
	}
	if err := validateKeyState(state, s.state.Scope, s.state.WorkspaceID); err != nil {
		s.destroy()
		return err
	}
	return nil
}

func (s *KeyService) ready() error {
	if err := s.check(); err != nil {
		return err
	}
	if s.key == nil {
		return ErrLocked
	}
	return nil
}

// Seal encrypts a record using SealRecord and this service's fixed scope.
func (s *KeyService) Seal(recordKey string, plaintext []byte) (Envelope, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return Envelope{}, err
	}
	return SealRecord(s.key, s.state.Scope, s.state.WorkspaceID, recordKey, plaintext)
}

// Open authenticates a record using OpenRecord and this service's fixed scope.
func (s *KeyService) Open(recordKey string, envelope Envelope) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return nil, err
	}
	return OpenRecord(s.key, s.state.Scope, s.state.WorkspaceID, recordKey, envelope)
}

// UnlockPasscode unwraps the WDK into memory only. Wrong passcodes and tampered
// wraps preserve all durable bytes and return keyring's typed unwrap errors.
func (s *KeyService) UnlockPasscode(passcode string) error {
	return s.unlock(keyring.WrapPasscode, keyring.UnwrapSecret{Passcode: passcode, Deriver: s.deriver})
}

// UnlockRecovery opens the existing recovery wrap without changing storage.
// RepairDevice can then restore a missing device key explicitly.
func (s *KeyService) UnlockRecovery(code string) error {
	return s.unlock(keyring.WrapRecovery, keyring.UnwrapSecret{RecoveryCode: code})
}

func (s *KeyService) unlock(kind keyring.WrapKind, secret keyring.UnwrapSecret) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.check(); err != nil {
		return err
	}
	wk, err := s.state.Ring.Unwrap(kind, "", secret)
	if err != nil {
		return fmt.Errorf("atrest: unlock: %w", err)
	}
	s.destroy()
	s.key = wk
	return nil
}

func (s *KeyService) commit(next *KeyState, device keyring.Wrapper) error {
	if s.state.Revision == ^uint64(0) {
		return ErrInvalidKeyState
	}
	next.Revision = s.state.Revision + 1
	if err := validateKeyState(next, s.state.Scope, s.state.WorkspaceID); err != nil {
		return err
	}
	if err := s.store.Commit(s.state.Revision, next, device); err != nil {
		if errors.Is(err, ErrKeyStateChanged) {
			s.destroy()
		}
		return fmt.Errorf("atrest: commit keys: %w", err)
	}
	s.state, s.device = next, device
	return nil
}

// EnableAppLock replaces the device-only wrap with a passcode wrap atomically.
// The current session stays unlocked; Lock discards its in-memory key.
func (s *KeyService) EnableAppLock(passcode string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return err
	}
	if s.state.AppLock {
		return fmt.Errorf("%w: app lock already enabled", ErrInvalidKeyState)
	}
	next := s.state.Clone()
	if err := next.Ring.AddPasscodeWrap(s.key, passcode, s.deriver); err != nil {
		return err
	}
	if err := next.Ring.RemoveWrap(s.key.Version(), keyring.WrapDevice, "", false); err != nil {
		return err
	}
	next.AppLock = true
	return s.commit(next, s.device)
}

// DisableAppLock restores a device wrap and removes the passcode wrap atomically.
// The service must already be unlocked by passcode or recovery.
func (s *KeyService) DisableAppLock() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return err
	}
	if !s.state.AppLock {
		return fmt.Errorf("%w: app lock is already disabled", ErrInvalidKeyState)
	}
	if s.device == nil {
		return errors.Join(ErrRecoveryRequired, ErrMissingDeviceKey)
	}
	next := s.state.Clone()
	if err := next.Ring.AddDeviceWrap(s.key, "", s.device); err != nil {
		return err
	}
	if err := next.Ring.RemoveWrap(s.key.Version(), keyring.WrapPasscode, "", false); err != nil {
		return err
	}
	next.AppLock = false
	return s.commit(next, s.device)
}

// ChangePasscode rewraps the same WDK, leaving every data record unchanged.
// The service must already be unlocked; the new passcode must be nonempty.
func (s *KeyService) ChangePasscode(passcode string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return err
	}
	if !s.state.AppLock {
		return ErrInvalidKeyState
	}
	next := s.state.Clone()
	if err := next.Ring.RewrapPasscode(s.key, passcode, s.deriver); err != nil {
		return err
	}
	return s.commit(next, s.device)
}

// AddRecovery creates a recovery wrap and returns its code only after commit.
// The caller must show it once and must not persist the printable secret.
func (s *KeyService) AddRecovery() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return "", err
	}
	next := s.state.Clone()
	code, err := next.Ring.AddRecoveryWrap(s.key)
	if err != nil {
		return "", err
	}
	if err := s.commit(next, s.device); err != nil {
		return "", err
	}
	return code, nil
}

// RepairDevice explicitly creates a replacement device key after a successful
// passcode/recovery unlock. App Lock stays enabled when it was enabled; in that
// case no device wrap is added. Existing ciphertext and the WDK never change.
func (s *KeyService) RepairDevice() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return err
	}
	if s.device != nil {
		return fmt.Errorf("%w: device key already exists", ErrInvalidKeyState)
	}
	device, err := s.store.NewDevice()
	if err != nil {
		return fmt.Errorf("atrest: repair device key: %w", err)
	}
	next := s.state.Clone()
	if !next.AppLock {
		entry, err := keyring.WrapWithDevice(s.key, "", device)
		if err != nil {
			return err
		}
		if err := next.Ring.Rewrap(s.key, entry); err != nil {
			return err
		}
	}
	return s.commit(next, device)
}
