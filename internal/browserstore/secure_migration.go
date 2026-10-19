//go:build js && wasm

package browserstore

import (
	"fmt"
	"log/slog"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/aicred"
	"github.com/monstercameron/CashFlux/internal/atrest"
)

var secureInitWaitLimit = 90 * time.Second

func initializeSecureBounded() error {
	slog.Info("protected storage boot stage", "stage", "key and migration verification", "status", "started")
	done := make(chan error, 1)
	go func() { done <- initializeSecure() }()
	select {
	case err := <-done:
		slog.Info("protected storage boot stage", "stage", "key and migration verification", "status", "complete", "error", err)
		return err
	case <-time.After(secureInitWaitLimit):
		return fmt.Errorf("protected storage key/migration verification did not finish within %s; saved records were kept", secureInitWaitLimit)
	}
}

func initializeSecure() error {
	// Include both databases' legacy values and staged encrypted migration copies
	// before allowing key creation. Missing key metadata must never replace keys.
	keys, err := (idbRecords{}).Keys()
	if err != nil {
		return err
	}
	slog.Info("protected storage boot stage", "stage", "inspect records", "records", len(keys))
	existing := false
	for _, key := range keys {
		if recoveryValueKey(strings.TrimPrefix(key, atrest.MigrationPrefix)) {
			// Opaque recovery data is evidence of an existing key.
			// Never create replacement keys after losing the live checkpoint.
			existing = true
			continue
		}
		value, _, err := (idbRecords{}).Read(key)
		if err != nil {
			return err
		}
		if atrest.IsSealed(value) {
			existing = true
		}
	}
	legacy, err := legacyValues()
	if err != nil {
		return err
	}
	for _, value := range legacy {
		if atrest.IsSealed(value) {
			existing = true
		}
	}
	if secureKeys != nil {
		secureKeys.Lock()
		secureKeys = nil
	}
	if secureKeyStore != nil {
		secureKeyStore.Close()
		secureKeyStore = nil
	}
	secureKeyStore, err = atrest.NewBrowserKeyStore(secureScope, secureWorkspace)
	if err != nil {
		return err
	}
	secureKeys, err = atrest.OpenKeyService(secureKeyStore, secureScope, secureWorkspace, existing)
	// Retain a non-nil service on recovery errors, for explicit recovery UI.
	if err != nil {
		return err
	}
	if secureKeys.State().AppLock {
		secureLocked = true
		return atrest.ErrLocked
	}
	return finishSecureMigration()
}

func finishSecureMigration() error {
	keys, err := (idbRecords{}).Keys()
	if err != nil {
		return err
	}
	legacy, err := legacyValues()
	if err != nil {
		return err
	}
	slog.Info("protected storage boot stage", "stage", "migrate records", "records", len(keys))
	active := storage()
	active.Backend = bootRecords{}
	migratedKeys, err = active.Migrate()
	if err != nil {
		return err
	}
	if err := migrateTransientKeys(keys); err != nil {
		return err
	}
	if err := importLegacyValues(legacy); err != nil {
		return err
	}
	slog.Info("protected storage boot stage", "stage", "load authenticated cache")
	return loadSecureCache()
}

func loadSecureCache() error {
	keys, err := (idbRecords{}).Keys()
	if err != nil {
		return err
	}
	values := map[string]string{}
	deferred := map[string]bool{}
	sliceStart := time.Now()
	for _, key := range keys {
		if time.Since(sliceStart) >= 12*time.Millisecond {
			time.Sleep(time.Millisecond)
			sliceStart = time.Now()
		}
		if recoveryValueKey(key) {
			deferred[key] = true
			continue
		}
		if strings.HasPrefix(key, atrest.MigrationPrefix) {
			continue
		}
		encoded, present, err := (idbRecords{}).Read(key)
		if err != nil {
			return &RecordError{Key: key, Err: err}
		}
		if present {
			value, err := decodeSecureRecord(key, encoded)
			if err != nil {
				return &RecordError{Key: key, Err: err}
			}
			values[key] = value
		}
	}
	mu.Lock()
	for key, value := range cache {
		if atrest.Classify(key) == atrest.ShouldNotPersist {
			values[key] = value
		}
	}
	cache = values
	deferredKeys = deferred
	readErrors = map[string]error{}
	mu.Unlock()
	slog.Info("protected storage boot stage", "stage", "load authenticated cache", "status", "complete", "records", len(values), "deferred", len(deferred))
	return nil
}

// bootRecords leaves recovery originals and staged migrations untouched until
// explicitly opened. It never reads their values during ordinary boot.
type bootRecords struct{ idbRecords }

func (bootRecords) Keys() ([]string, error) {
	keys, err := (idbRecords{}).Keys()
	active := keys[:0]
	for _, key := range keys {
		if !recoveryValueKey(strings.TrimPrefix(key, atrest.MigrationPrefix)) {
			active = append(active, key)
		}
	}
	return active, err
}

func legacyValues() (values map[string]string, err error) {
	values = map[string]string{}
	if idbName != "cashflux-kv" {
		return values, nil
	}
	defer func() {
		if r := recover(); r != nil {
			err = jsPanicError("read legacy browser storage", r)
		}
	}()
	ls := js.Global().Get("localStorage")
	if !ls.Truthy() {
		return values, nil
	}
	for i := 0; i < ls.Get("length").Int(); i++ {
		key := ls.Call("key", i).String()
		if strings.HasPrefix(key, keyPrefix) {
			values[key] = ls.Call("getItem", key).String()
		}
	}
	return values, nil
}

func importLegacyValues(values map[string]string) error {
	for key, value := range values {
		if atrest.Classify(key) == atrest.ShouldNotPersist {
			if err := retainTransient(key, value); err != nil {
				return err
			}
		} else {
			stored, present, err := (idbRecords{}).Read(key)
			if err != nil {
				return err
			}
			if present {
				plain, err := storage().Decode(key, stored)
				if err != nil {
					return err
				}
				if plain != value {
					return fmt.Errorf("legacy copy differs for %q: %w", key, atrest.ErrRecordChanged)
				}
			} else {
				if atrest.IsSealed(value) {
					return atrest.ErrRecoveryRequired
				}
				if err := storage().Set(key, value); err != nil {
					return err
				}
				plain, have, err := storage().Get(key)
				if err != nil {
					return err
				}
				if !have || plain != value {
					return atrest.ErrRecordChanged
				}
				migratedKeys = append(migratedKeys, key)
			}
		}
		// The legacy source is deleted only after durable write and read-back.
		if err := removeLegacy(key, value); err != nil {
			return err
		}
	}
	return nil
}

func removeLegacy(key, expected string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = jsPanicError("remove verified legacy value", r)
		}
	}()
	ls := js.Global().Get("localStorage")
	if ls.Call("getItem", key).String() != expected {
		return atrest.ErrRecordChanged
	}
	ls.Call("removeItem", key)
	return nil
}

func migrateTransientKeys(keys []string) error {
	for _, key := range keys {
		if atrest.Classify(key) != atrest.ShouldNotPersist {
			continue
		}
		value, present, err := (idbRecords{}).Read(key)
		if err != nil {
			return err
		}
		if !present {
			continue
		}
		if err := retainTransient(key, value); err != nil {
			return err
		}
		if err := (idbRecords{}).Delete(key, &value); err != nil {
			return err
		}
	}
	return nil
}

func retainTransient(key, value string) error {
	// Preserve a remembered legacy OpenAI credential durably in the approved
	// encrypted record BEFORE deleting the raw credential migration source.
	if key == "cashflux:openai-key" && strings.TrimSpace(value) != "" {
		const target = "cashflux:cred:openai"
		raw, present, err := storage().Get(target)
		if err != nil {
			return err
		}
		if present {
			record, err := aicred.Parse([]byte(raw))
			if err != nil {
				return err
			}
			if record.Key != strings.TrimSpace(value) {
				return atrest.ErrRecordChanged
			}
		} else {
			record, err := aicred.New(strings.TrimSpace(value), "", true)
			if err != nil {
				return err
			}
			raw, err := aicred.Marshal(record)
			if err != nil {
				return err
			}
			if err := storage().Set(target, string(raw)); err != nil {
				return err
			}
			verified, have, err := storage().Get(target)
			if err != nil {
				return err
			}
			if !have || verified != string(raw) {
				return atrest.ErrRecordChanged
			}
		}
	}
	SetMemory(key, value)
	return nil
}
