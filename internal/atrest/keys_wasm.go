//go:build js && wasm

package atrest

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/keyring"
)

// BrowserKeyStore persists a workspace's key state and non-extractable device
// CryptoKey together in IndexedDB. There is no localStorage fallback. Methods
// wait for browser events and must be called from a Go goroutine.
type BrowserKeyStore struct {
	db          js.Value
	scope       keyring.Scope
	workspaceID string
}

// NewBrowserKeyStore opens an isolated key database for exactly this scope and
// workspace. It does not create key material or touch application record stores.
// Close it when the owning page/scope is closed.
func NewBrowserKeyStore(scope keyring.Scope, workspaceID string) (store *BrowserKeyStore, err error) {
	defer browserError(&err)
	if scope.ServerID == "" || (!scope.IsLocal() && scope.AccountID == "") || workspaceID == "" {
		return nil, ErrInvalidKeyState
	}
	idb := js.Global().Get("indexedDB")
	if !idb.Truthy() {
		return nil, fmt.Errorf("atrest: IndexedDB unavailable")
	}
	identity, err := json.Marshal([]string{scope.ServerID, scope.AccountID, workspaceID})
	if err != nil {
		return nil, fmt.Errorf("atrest: encode key database identity: %w", err)
	}
	name := "cashflux-atrest-keys-v1:" + base64.RawURLEncoding.EncodeToString(identity)
	req := idb.Call("open", name, 1)
	done := make(chan error, 1)
	upgrade := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		var upgradeErr error
		func() { defer browserError(&upgradeErr); req.Get("result").Call("createObjectStore", "keys") }()
		if upgradeErr != nil {
			req.Get("transaction").Call("abort")
		}
		return nil
	})
	ok := js.FuncOf(func(_ js.Value, _ []js.Value) any { done <- nil; return nil })
	fail := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		done <- jsFailure("open key database", req.Get("error"))
		return nil
	})
	defer upgrade.Release()
	defer ok.Release()
	defer fail.Release()
	req.Set("onupgradeneeded", upgrade)
	req.Set("onsuccess", ok)
	req.Set("onerror", fail)
	if err := <-done; err != nil {
		return nil, err
	}
	return &BrowserKeyStore{db: req.Get("result"), scope: scope, workspaceID: workspaceID}, nil
}

// Close releases the database connection. Existing ciphertext is preserved.
func (s *BrowserKeyStore) Close() { s.db.Call("close") }

// Load reads key metadata and the CryptoKey in one readonly transaction.
func (s *BrowserKeyStore) Load() (*KeyState, keyring.Wrapper, error) {
	value, err := s.transaction(nil, nil, nil)
	if err != nil {
		return nil, nil, err
	}
	if value.IsUndefined() || value.IsNull() {
		return nil, nil, nil
	}
	if value.Type() != js.TypeObject || value.Get("state").Type() != js.TypeString {
		return nil, nil, ErrInvalidKeyState
	}
	var state KeyState
	if err := json.Unmarshal([]byte(value.Get("state").String()), &state); err != nil {
		return nil, nil, fmt.Errorf("%w: decode key state: %w", ErrInvalidKeyState, err)
	}
	if err := validateKeyState(&state, s.scope, s.workspaceID); err != nil {
		return nil, nil, err
	}
	device := value.Get("deviceKey")
	if device.IsUndefined() || device.IsNull() {
		return &state, nil, nil
	}
	if !validDeviceKey(device) {
		return nil, nil, fmt.Errorf("%w: stored device key is not a non-extractable AES-256-GCM CryptoKey", ErrInvalidKeyState)
	}
	return &state, &browserDevice{key: device}, nil
}

// NewDevice generates a non-extractable AES-256-GCM CryptoKey. Raw key bytes are
// never exported, copied into Go, or serialized. Commit stores the object using
// IndexedDB structured clone, which preserves its non-extractability.
func (s *BrowserKeyStore) NewDevice() (device keyring.Wrapper, err error) {
	defer browserError(&err)
	key, err := awaitCrypto(func() js.Value {
		return cryptoSubtle().Call("generateKey", map[string]any{"name": "AES-GCM", "length": 256}, false, []any{"encrypt", "decrypt"})
	})
	if err != nil {
		return nil, fmt.Errorf("atrest: generate device key: %w", err)
	}
	if !validDeviceKey(key) {
		return nil, ErrInvalidKeyState
	}
	return &browserDevice{key: key}, nil
}

// Commit uses a strict-durability transaction for the CAS and both stored
// values. Request success is insufficient: only transaction completion returns
// success. Aborts, quota failures, and CAS conflicts preserve the previous row.
func (s *BrowserKeyStore) Commit(expected uint64, state *KeyState, device keyring.Wrapper) error {
	if state == nil || expected == ^uint64(0) || state.Revision != expected+1 {
		return ErrInvalidKeyState
	}
	if err := validateKeyState(state, s.scope, s.workspaceID); err != nil {
		return err
	}
	if device != nil {
		wrapped, ok := device.(*browserDevice)
		if !ok || !validDeviceKey(wrapped.key) {
			return ErrInvalidKeyState
		}
	}
	_, err := s.transaction(&expected, state, device)
	return err
}

// transaction deliberately queues put inside get.onsuccess, with no Promise,
// channel wait, or goroutine handoff inside that callback: real IndexedDB marks
// a transaction inactive as soon as its request-event task returns.
func (s *BrowserKeyStore) transaction(expected *uint64, state *KeyState, device keyring.Wrapper) (value js.Value, err error) {
	defer browserError(&err)
	mode := "readonly"
	var encoded []byte
	if expected != nil {
		mode = "readwrite"
		encoded, err = json.Marshal(state)
		if err != nil {
			return js.Undefined(), fmt.Errorf("atrest: encode key state: %w", err)
		}
	}
	tx := s.db.Call("transaction", "keys", mode, map[string]any{"durability": "strict"})
	objectStore := tx.Call("objectStore", "keys")
	var operationErr error
	done := make(chan error, 1)
	complete := js.FuncOf(func(_ js.Value, _ []js.Value) any { done <- operationErr; return nil })
	abort := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		if operationErr != nil {
			done <- operationErr
		} else {
			done <- jsFailure("key transaction aborted", tx.Get("error"))
		}
		return nil
	})
	success := js.FuncOf(func(_ js.Value, args []js.Value) any {
		func() {
			defer browserError(&operationErr)
			value = args[0].Get("target").Get("result")
			if expected == nil {
				return
			}
			var revision uint64
			if !value.IsUndefined() && !value.IsNull() {
				if *expected == 0 {
					operationErr = ErrKeyStateChanged
					return
				}
				if value.Type() != js.TypeObject || value.Get("state").Type() != js.TypeString {
					operationErr = ErrInvalidKeyState
					return
				}
				var old KeyState
				if decodeErr := json.Unmarshal([]byte(value.Get("state").String()), &old); decodeErr != nil {
					operationErr = fmt.Errorf("%w: decode previous key state: %w", ErrInvalidKeyState, decodeErr)
					return
				}
				if invalid := validateKeyState(&old, s.scope, s.workspaceID); invalid != nil {
					operationErr = invalid
					return
				}
				oldDevice := value.Get("deviceKey")
				if !oldDevice.IsUndefined() && !oldDevice.IsNull() && !validDeviceKey(oldDevice) {
					operationErr = ErrInvalidKeyState
					return
				}
				revision = old.Revision
			}
			if revision != *expected {
				operationErr = ErrKeyStateChanged
				return
			}
			row := map[string]any{"state": string(encoded)}
			if device != nil {
				row["deviceKey"] = device.(*browserDevice).key
			}
			objectStore.Call("put", row, "workspace")
		}()
		if operationErr != nil {
			tx.Call("abort")
		}
		return nil
	})
	defer complete.Release()
	defer abort.Release()
	defer success.Release()
	tx.Set("oncomplete", complete)
	tx.Set("onabort", abort)
	req := objectStore.Call("get", "workspace")
	req.Set("onsuccess", success)
	err = <-done
	return value, err
}

type browserDevice struct{ key js.Value }

func validDeviceKey(key js.Value) (valid bool) {
	defer func() {
		if recover() != nil {
			valid = false
		}
	}()
	// instanceof is realm-sensitive; the properties plus WebCrypto's own brand
	// checks on use reject invalid keys without assuming a global CryptoKey.
	if key.Type() != js.TypeObject || key.IsNull() {
		return false
	}
	algorithm := key.Get("algorithm")
	if algorithm.Type() != js.TypeObject || algorithm.IsNull() {
		return false
	}
	if key.Get("type").String() != "secret" || key.Get("extractable").Type() != js.TypeBoolean || key.Get("extractable").Bool() || algorithm.Get("name").String() != "AES-GCM" || algorithm.Get("length").Type() != js.TypeNumber || algorithm.Get("length").Int() != 256 {
		return false
	}
	usages := key.Get("usages")
	return usages.Truthy() && usages.Call("includes", "encrypt").Bool() && usages.Call("includes", "decrypt").Bool()
}

func (d *browserDevice) Wrap(plain, aad []byte) ([]byte, []byte, error) {
	nonce := make([]byte, keyring.NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, fmt.Errorf("atrest: device nonce: %w", err)
	}
	ciphertext, err := d.crypt("encrypt", nonce, plain, aad)
	return nonce, ciphertext, err
}

func (d *browserDevice) Unwrap(nonce, ciphertext, aad []byte) ([]byte, error) {
	plain, err := d.crypt("decrypt", nonce, ciphertext, aad)
	if err != nil {
		return nil, &keyring.AuthenticationError{Operation: "device unwrap", Err: err}
	}
	return plain, nil
}

func (d *browserDevice) crypt(operation string, nonce, input, aad []byte) ([]byte, error) {
	inputArray := bytesToJS(input)
	defer inputArray.Call("fill", 0)
	value, err := awaitCrypto(func() js.Value {
		return cryptoSubtle().Call(operation, map[string]any{"name": "AES-GCM", "iv": bytesToJS(nonce), "additionalData": bytesToJS(aad), "tagLength": 128}, d.key, inputArray)
	})
	if err != nil {
		return nil, err
	}
	array := js.Global().Get("Uint8Array").New(value)
	defer array.Call("fill", 0)
	out := make([]byte, array.Get("length").Int())
	js.CopyBytesToGo(out, array)
	return out, nil
}

func bytesToJS(raw []byte) js.Value {
	array := js.Global().Get("Uint8Array").New(len(raw))
	js.CopyBytesToJS(array, raw)
	return array
}

func cryptoSubtle() js.Value { return js.Global().Get("crypto").Get("subtle") }

func awaitCrypto(start func() js.Value) (value js.Value, err error) {
	defer browserError(&err)
	done := make(chan error, 1)
	ok := js.FuncOf(func(_ js.Value, args []js.Value) any { value = args[0]; done <- nil; return nil })
	fail := js.FuncOf(func(_ js.Value, args []js.Value) any { done <- jsFailure("WebCrypto", args[0]); return nil })
	defer ok.Release()
	defer fail.Release()
	start().Call("then", ok, fail)
	err = <-done
	return value, err
}

func browserError(err *error) {
	if r := recover(); r != nil {
		*err = fmt.Errorf("atrest: browser operation failed: %v", r)
	}
}

func jsFailure(operation string, value js.Value) error {
	return fmt.Errorf("atrest: %s: %s", operation, value.String())
}

type browserDeriver struct{}

func (browserDeriver) DerivePBKDF2(password string, salt []byte, iterations, keyLen int) (key []byte, err error) {
	defer browserError(&err)
	type result struct {
		key []byte
		err error
	}
	done := make(chan result, 1)
	keyring.WebCryptoPBKDF2(password, salt, iterations, keyLen, func(key []byte, err error) { done <- result{key, err} })
	out := <-done
	return out.key, out.err
}

func platformDeriver() keyring.PBKDF2Deriver { return browserDeriver{} }
