//go:build js && wasm

package browserstore

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/atrest"
	"github.com/monstercameron/CashFlux/internal/keyring"
)

var (
	secureScope = keyring.LocalScope("")
	// v1 stores the whole installation, including inactive workspace bundles,
	// in a single database. This is its explicit local migration scope. v2 boot
	// must select its fixed account/workspace through ConfigureScope before Init.
	secureWorkspace = "legacy-browser-profile"
	secureKeys      *atrest.KeyService
	secureKeyStore  *atrest.BrowserKeyStore
	secureInitErr   error
	readErrors      = map[string]error{}
	secureLocked    bool
	queueMu         sync.Mutex
	queue           []func()
	queueRunning    bool
	migratedKeys    []string
)

// RecordError identifies an unreadable protected record without returning an
// empty value. It preserves locked, recovery and authentication error identities.
type RecordError struct {
	// Key names the protected record that could not be read.
	Key string
	// Err preserves the underlying storage or key-service failure.
	Err error
}

// Error describes the failed protected-storage operation.
func (e *RecordError) Error() string { return fmt.Sprintf("protected storage %q: %v", e.Key, e.Err) }

// Unwrap preserves errors.Is checks for the underlying key-service error.
func (e *RecordError) Unwrap() error { return e.Err }

// ConfigureScope selects the immutable key scope before Init. A running page
// cannot change scope; identity changes require closing the page and reloading.
// Scoped records use a separate physical database as well as authenticated AAD.
func ConfigureScope(scope keyring.Scope, workspace string) error {
	if ready || secureKeys != nil || workspace == "" || scope.ServerID == "" {
		return atrest.ErrInvalidKeyState
	}
	secureScope, secureWorkspace = scope, workspace
	raw, err := json.Marshal([]string{scope.ServerID, scope.AccountID, workspace})
	if err != nil {
		return err
	}
	idbName = fmt.Sprintf("cashflux-kv-v2:%x", sha256.Sum256(raw))
	return nil
}

func storage() atrest.Storage {
	s := atrest.Storage{Backend: idbRecords{}}
	if secureKeys != nil {
		s.Cipher = secureKeys
	}
	return s
}

// InitError reports a boot/migration failure. Callers must present recovery and
// must not seed or autosave an empty replacement while this error is non-nil.
func InitError() error { return secureInitErr }

// MigratedKeys returns the successfully migrated key names, never their values.
func MigratedKeys() []string { return append([]string(nil), migratedKeys...) }

// SecureKeyState returns a copy of public key metadata for the pre-hydration
// lock/recovery gate. It never exposes the device key, WDK or decrypted records.
func SecureKeyState() *atrest.KeyState {
	if secureKeys == nil {
		return nil
	}
	return secureKeys.State()
}

func cachedError(key string) error {
	if atrest.Classify(key) != atrest.MustEncrypt {
		return nil
	}
	if secureInitErr != nil {
		return secureInitErr
	}
	if secureLocked {
		return atrest.ErrLocked
	}
	return readErrors[key]
}

// SetMemory keeps transient credentials only in this page's memory. It never
// publishes their values or creates an IndexedDB/localStorage record.
func SetMemory(key, value string) { mu.Lock(); cache[key] = value; mu.Unlock() }

func logWriteError(key string) func(error) {
	return func(err error) {
		if err != nil {
			slog.Error("protected storage write failed", "key", key, "error", err)
		}
	}
}

func enqueue(fn func()) {
	queueMu.Lock()
	queue = append(queue, fn)
	if queueRunning {
		queueMu.Unlock()
		return
	}
	queueRunning = true
	queueMu.Unlock()
	go func() {
		for {
			queueMu.Lock()
			if len(queue) == 0 {
				queueRunning = false
				queueMu.Unlock()
				return
			}
			job := queue[0]
			queue[0] = nil
			queue = queue[1:]
			queueMu.Unlock()
			job()
		}
	}()
}

// SetSecure encrypts according to the catalogue and reports success only after
// the strict IndexedDB commit. It is nonblocking and safe in a JS event handler.
// SHOULD-NOT-PERSIST keys are refused; use SetMemory for transient credentials.
func SetSecure(key, value string, done func(error)) {
	SetSecureBatch(map[string]string{key: value}, nil, done)
}

// SetSecureBatch commits related records atomically. expected names plaintext
// values the caller is entitled to replace; an absent record matches "". This
// lets dataset writers compare their generation without a wall-clock winner.
// Conditions may name read-only records, which are compared without rewriting.
// The maps are copied before return, so the caller may safely reuse them.
func SetSecureBatch(values, expected map[string]string, done func(error)) {
	setSecureBatch(values, expected, nil, done)
}

func setSecureBatch(values, expected map[string]string, ciphertexts map[string]*string, done func(error)) {
	updates := make(map[string]string, len(values))
	conditions := make(map[string]string, len(expected))
	for key, value := range values {
		updates[key] = value
	}
	for key, value := range expected {
		conditions[key] = value
	}
	enqueue(func() {
		err := secureInitErr
		mu.Lock()
		locked := secureLocked
		mu.Unlock()
		if locked {
			for key := range updates {
				if atrest.Classify(key) == atrest.MustEncrypt {
					err = atrest.ErrLocked
					break
				}
			}
		}
		if err == nil {
			err = writeSecureBatchCompared(updates, conditions, ciphertexts)
		}
		if err == nil {
			mu.Lock()
			for key, value := range updates {
				if pending[key] == 0 && (!secureLocked || atrest.Classify(key) == atrest.MayStayPlain) {
					cache[key] = value
				}
				delete(readErrors, key)
			}
			mu.Unlock()
			keys := make([]string, 0, len(updates))
			for key := range updates {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			publishBatch(keys)
		}
		if done != nil {
			done(err)
		}
	})
}

type recordReplacement struct {
	key, value string
	expected   *string
	write      bool
}

func recordChangedAt(key, stage string) error {
	return fmt.Errorf("protected record %q changed during %s: %w", key, stage, atrest.ErrRecordChanged)
}

func writeSecureBatch(values, expected map[string]string) error {
	return writeSecureBatchCompared(values, expected, nil)
}

func writeSecureBatchCompared(values, expected map[string]string, ciphertexts map[string]*string) error {
	var keys []string
	for key := range ciphertexts {
		if _, write := values[key]; !write {
			if _, plain := expected[key]; !plain {
				keys = append(keys, key)
			}
		}
	}
	for key := range expected {
		if _, have := values[key]; !have {
			keys = append(keys, key)
		}
	}
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	priorRecords, err := readSecureBatch(keys)
	if err != nil {
		return err
	}
	updates := make([]recordReplacement, 0, len(keys))
	sliceStart := time.Now()
	for i, key := range keys {
		old, have := priorRecords[key]
		if want, conditional := ciphertexts[key]; conditional {
			if (want == nil && have) || (want != nil && (!have || old != *want)) {
				return recordChangedAt(key, "snapshot ciphertext condition")
			}
		}
		if want, conditional := expected[key]; conditional {
			plain := ""
			if have {
				plain, err = storage().Decode(key, old)
				if err != nil {
					return err
				}
			}
			if plain != want {
				return recordChangedAt(key, "plaintext generation condition")
			}
		}
		plain, write := values[key]
		var value string
		if write {
			value, err = storage().Encode(key, plain)
			if err != nil {
				return err
			}
		}
		var prior *string
		if have {
			copy := old
			prior = &copy
		}
		updates = append(updates, recordReplacement{key: key, value: value, expected: prior, write: write})
		// Encryption and JSON encoding run outside the strict transaction. Give
		// the browser a timer boundary before a large journal monopolizes a task.
		if i+1 < len(keys) && time.Since(sliceStart) >= 12*time.Millisecond {
			time.Sleep(time.Millisecond)
			sliceStart = time.Now()
		}
	}
	return recordTransaction(true, func(tx js.Value, retain func(js.Func), fail func(error)) {
		os := tx.Call("objectStore", idbStoreName)
		for _, update := range updates {
			u := update
			req := os.Call("get", u.key)
			fn := js.FuncOf(func(js.Value, []js.Value) any {
				defer func() {
					if r := recover(); r != nil {
						fail(jsPanicError("write protected batch", r))
						tx.Call("abort")
					}
				}()
				v := req.Get("result")
				present := !v.IsUndefined() && !v.IsNull()
				if (u.expected == nil && present) || (u.expected != nil && (!present || v.Type() != js.TypeString || v.String() != *u.expected)) {
					fail(recordChangedAt(u.key, "encrypted prior-record comparison"))
					tx.Call("abort")
					return nil
				}
				if u.write {
					os.Call("put", u.value, u.key)
				}
				return nil
			})
			retain(fn)
			req.Set("onsuccess", fn)
		}
	})
}

// RemoveSecure deletes a record durably before updating the cache or calling
// done. A failed or aborted transaction is never acknowledged as success.
func RemoveSecure(key string, done func(error)) {
	enqueue(func() {
		err := secureInitErr
		if err == nil {
			err = storage().Remove(key)
		}
		if err == nil {
			mu.Lock()
			if pending[key] == 0 {
				delete(cache, key)
			}
			delete(readErrors, key)
			mu.Unlock()
			publish(key)
		}
		if done != nil {
			done(err)
		}
	})
}

// GetSecure authenticates a durable value and returns a typed error while
// locked. This method BLOCKS and must run from a Go goroutine, never a JS callback.
func GetSecure(key string) (string, bool, error) {
	if atrest.Classify(key) == atrest.ShouldNotPersist {
		mu.Lock()
		value, present := cache[key]
		mu.Unlock()
		return value, present, nil
	}
	if secureInitErr != nil {
		return "", false, &RecordError{Key: key, Err: secureInitErr}
	}
	mu.Lock()
	locked := secureLocked
	mu.Unlock()
	if locked && atrest.Classify(key) == atrest.MustEncrypt {
		return "", true, &RecordError{Key: key, Err: atrest.ErrLocked}
	}
	value, present, err := storage().Get(key)
	if err != nil {
		return "", present, &RecordError{Key: key, Err: err}
	}
	return value, present, nil
}

// LockSecure destroys the cached WDK and clears decrypted sensitive values.
// It must be called at the app's lock/scope-close boundary.
func LockSecure() {
	LockSecureThen(nil)
}

// LockSecureThen stops cache reads immediately and destroys the WDK after prior
// queued writes settle. It never blocks a JavaScript event callback on key I/O.
// done runs only after the in-memory WDK has been destroyed.
func LockSecureThen(done func()) {
	mu.Lock()
	secureLocked = true
	for key := range cache {
		if atrest.Classify(key) != atrest.MayStayPlain {
			delete(cache, key)
		}
	}
	mu.Unlock()
	enqueue(func() {
		if secureKeys != nil {
			secureKeys.Lock()
		}
		if done != nil {
			done()
		}
	})
}

// UnlockSecure unlocks the existing WDK and reloads protected records. It blocks;
// run it from a goroutine and acknowledge unlock only after it returns nil.
func UnlockSecure(passcode string) error {
	done := make(chan error, 1)
	UnlockSecureThen(passcode, func(err error) { done <- err })
	return <-done
}

// UnlockSecureThen unlocks and completes any interrupted migration before
// publishing decrypted cache values. It is safe from a JavaScript callback.
func UnlockSecureThen(passcode string, done func(error)) {
	unlockSecure(func(s *atrest.KeyService) error { return s.UnlockPasscode(passcode) }, done)
}

// UnlockRecoverySecure opens the retained keyring with an explicitly supplied
// recovery code. It never persists the code or automatically repairs the device.
func UnlockRecoverySecure(code string, done func(error)) {
	unlockSecure(func(s *atrest.KeyService) error { return s.UnlockRecovery(code) }, done)
}

// RepairDeviceSecure explicitly repairs a device key after successful unlock.
// The same WDK and existing ciphertext are retained; errors never reset data.
func RepairDeviceSecure(done func(error)) {
	keyAction(func(s *atrest.KeyService) error { return s.RepairDevice() }, done)
}

func unlockSecure(action func(*atrest.KeyService) error, done func(error)) {
	enqueue(func() {
		err := atrest.ErrRecoveryRequired
		if secureKeys != nil {
			err = action(secureKeys)
			if err == nil {
				err = finishSecureMigration()
			}
		}
		if err == nil {
			secureInitErr = nil
			mu.Lock()
			secureLocked = false
			mu.Unlock()
		}
		if done != nil {
			done(err)
		}
	})
}

// EnableAppLockSecure durably replaces the device wrap with a passcode wrap.
// It leaves the current session unlocked; use LockSecureThen for immediate lock.
func EnableAppLockSecure(passcode string, done func(error)) {
	keyAction(func(s *atrest.KeyService) error { return s.EnableAppLock(passcode) }, done)
}

// ChangePasscodeSecure durably rewraps the same WDK without rewriting records.
func ChangePasscodeSecure(passcode string, done func(error)) {
	keyAction(func(s *atrest.KeyService) error { return s.ChangePasscode(passcode) }, done)
}

// DisableAppLockSecure durably restores the device wrap of the existing WDK.
func DisableAppLockSecure(done func(error)) {
	keyAction(func(s *atrest.KeyService) error { return s.DisableAppLock() }, done)
}

func keyAction(action func(*atrest.KeyService) error, done func(error)) {
	enqueue(func() {
		err := secureInitErr
		if err == nil {
			if secureKeys == nil {
				err = atrest.ErrRecoveryRequired
			} else {
				err = action(secureKeys)
			}
		}
		if done != nil {
			done(err)
		}
	})
}
