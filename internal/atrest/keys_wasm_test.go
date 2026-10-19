//go:build js && wasm

package atrest

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"syscall/js"
	"testing"

	"github.com/monstercameron/CashFlux/internal/keyring"
)

var browserKeysCounter int

func setupBrowserKeys(t *testing.T) (keyring.Scope, string) {
	t.Helper()
	crypto := js.Global().Get("crypto")
	if !crypto.Truthy() || !crypto.Get("subtle").Truthy() {
		require := js.Global().Get("require")
		if require.Type() != js.TypeFunction {
			t.Fatal("WebCrypto unavailable")
		}
		js.Global().Set("crypto", require.Invoke("node:crypto").Get("webcrypto"))
	}
	if !js.Global().Get("indexedDB").Truthy() {
		// Reuse localdb's exact shim, rather than maintaining a second fake.
		// The Node wasm runner exposes its fs bindings to Go's os package.
		script, err := os.ReadFile("../localdb/testdata/fake_indexeddb.js")
		if err != nil {
			t.Fatalf("load shared IndexedDB shim: %v", err)
		}
		if js.Global().Get("structuredClone").Type() != js.TypeFunction {
			t.Fatal("CryptoKey tests need Node structuredClone")
		}
		js.Global().Call("eval", string(script))
	}
	browserKeysCounter++
	return keyring.Scope{ServerID: "enc2-test", AccountID: fmt.Sprintf("account-%d", browserKeysCounter)}, "workspace"
}

func openBrowserKeys(t *testing.T, scope keyring.Scope, workspace string, existing bool) (*BrowserKeyStore, *KeyService) {
	t.Helper()
	store, err := NewBrowserKeyStore(scope, workspace)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	s, err := OpenKeyService(store, scope, workspace, existing)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Lock)
	return store, s
}

func TestBrowserDeviceKeyStructuredCloneReload(t *testing.T) {
	scope, workspace := setupBrowserKeys(t)
	store, service := openBrowserKeys(t, scope, workspace, false)
	envelope, err := service.Seal("cashflux:dataset", []byte("household secret"))
	if err != nil {
		t.Fatal(err)
	}
	state, wrapper, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	device, ok := wrapper.(*browserDevice)
	if !ok || !validDeviceKey(device.key) || device.key.Get("extractable").Bool() {
		t.Fatal("persisted device key lost its non-extractable CryptoKey type")
	}
	if _, err := awaitCrypto(func() js.Value { return cryptoSubtle().Call("exportKey", "raw", device.key) }); err == nil {
		t.Fatal("stored device key was exportable")
	}
	if len(state.Ring.Current().Wraps) != 1 || state.Ring.Current().Wraps[0].Kind != keyring.WrapDevice {
		t.Fatal("first use did not persist a device-wrapped WDK")
	}
	material := service.key.Material()
	defer zero(material)
	if bytes.Contains(keyStateBytes(t, state), material) {
		t.Fatal("raw WDK persisted")
	}
	service.Lock()
	store.Close()
	_, reloaded := openBrowserKeys(t, scope, workspace, true)
	assertRecord(t, reloaded, envelope)
	if reloaded.State().Ring.Current().KeyID != envelope.KeyID {
		t.Fatal("reload replaced the WDK")
	}
	for _, other := range []struct {
		scope     keyring.Scope
		workspace string
	}{
		{keyring.Scope{ServerID: "other-server", AccountID: scope.AccountID}, workspace},
		{keyring.Scope{ServerID: scope.ServerID, AccountID: "other-account"}, workspace},
		{scope, "other-workspace"},
	} {
		_, foreign := openBrowserKeys(t, other.scope, other.workspace, false)
		if _, err := foreign.Open("cashflux:dataset", envelope); !errors.Is(err, ErrKeyMismatch) {
			t.Fatalf("foreign workspace opened data: %v", err)
		}
	}
}

func TestBrowserAppLockWebCryptoLifecycle(t *testing.T) {
	scope, workspace := setupBrowserKeys(t)
	store, service := openBrowserKeys(t, scope, workspace, false)
	envelope, err := service.Seal("cashflux:dataset", []byte("household secret"))
	if err != nil {
		t.Fatal(err)
	}
	if err := service.EnableAppLock("first"); err != nil {
		t.Fatal(err)
	}
	state, _, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	entry := state.Ring.Current().Wraps[0]
	if entry.Kind != keyring.WrapPasscode || entry.Iterations != 600000 || len(entry.Salt) != keyring.PasscodeSaltSize {
		t.Fatal("wrong passcode wrapping parameters")
	}
	service.Lock()
	_, reloaded := openBrowserKeys(t, scope, workspace, true)
	if _, err := reloaded.Open("cashflux:dataset", envelope); !errors.Is(err, ErrLocked) {
		t.Fatalf("reload bypassed App Lock: %v", err)
	}
	before := keyStateBytes(t, state)
	if err := reloaded.UnlockPasscode("wrong"); !errors.Is(err, keyring.ErrUnwrapFailed) {
		t.Fatalf("wrong passcode = %v", err)
	}
	unchanged, _, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, keyStateBytes(t, unchanged)) {
		t.Fatal("wrong passcode changed durable metadata")
	}
	if err := reloaded.UnlockPasscode("first"); err != nil {
		t.Fatal(err)
	}
	assertRecord(t, reloaded, envelope)
	if err := reloaded.ChangePasscode("second"); err != nil {
		t.Fatal(err)
	}
	reloaded.Lock()
	if err := reloaded.UnlockPasscode("first"); !errors.Is(err, keyring.ErrUnwrapFailed) {
		t.Fatalf("old passcode = %v", err)
	}
	if err := reloaded.UnlockPasscode("second"); err != nil {
		t.Fatal(err)
	}
	assertRecord(t, reloaded, envelope)
	if err := reloaded.DisableAppLock(); err != nil {
		t.Fatal(err)
	}
	_, automatic := openBrowserKeys(t, scope, workspace, true)
	assertRecord(t, automatic, envelope)
}

// mutateBrowserKeyRow keeps the read and write in a single request callback,
// just like the production CAS, and waits for transaction completion.
func mutateBrowserKeyRow(t *testing.T, store *BrowserKeyStore, mutate func(js.Value)) {
	t.Helper()
	tx := store.db.Call("transaction", "keys", "readwrite", map[string]any{"durability": "strict"})
	objectStore := tx.Call("objectStore", "keys")
	done := make(chan error, 1)
	ok := js.FuncOf(func(_ js.Value, _ []js.Value) any { done <- nil; return nil })
	fail := js.FuncOf(func(_ js.Value, _ []js.Value) any { done <- jsFailure("test mutation", tx.Get("error")); return nil })
	read := js.FuncOf(func(_ js.Value, args []js.Value) any {
		value := args[0].Get("target").Get("result")
		mutate(value)
		objectStore.Call("put", value, "workspace")
		return nil
	})
	defer ok.Release()
	defer fail.Release()
	defer read.Release()
	tx.Set("oncomplete", ok)
	tx.Set("onabort", fail)
	objectStore.Call("get", "workspace").Set("onsuccess", read)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestBrowserMissingDeviceRecoveryAndTamper(t *testing.T) {
	scope, workspace := setupBrowserKeys(t)
	store, service := openBrowserKeys(t, scope, workspace, false)
	envelope, err := service.Seal("cashflux:dataset", []byte("household secret"))
	if err != nil {
		t.Fatal(err)
	}
	code, err := service.AddRecovery()
	if err != nil {
		t.Fatal(err)
	}
	service.Lock()
	mutateBrowserKeyRow(t, store, func(row js.Value) { row.Set("deviceKey", js.Undefined()) })
	state, _, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	before := keyStateBytes(t, state)
	recovery, err := OpenKeyService(store, scope, workspace, true)
	if recovery == nil || !errors.Is(err, ErrMissingDeviceKey) || !errors.Is(err, ErrRecoveryRequired) {
		t.Fatalf("missing device = %v", err)
	}
	t.Cleanup(recovery.Lock)
	state, _, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, keyStateBytes(t, state)) {
		t.Fatal("missing device changed key metadata")
	}
	if err := recovery.UnlockRecovery(code); err != nil {
		t.Fatal(err)
	}
	assertRecord(t, recovery, envelope)
	if err := recovery.RepairDevice(); err != nil {
		t.Fatal(err)
	}
	_, reloaded := openBrowserKeys(t, scope, workspace, true)
	assertRecord(t, reloaded, envelope)
	// Persist an altered wrapped WDK. Opening must fail without resetting it.
	state, wrapper, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	state.Ring.Current().Wraps[0].Wrapped[0] ^= 1
	expected := state.Revision
	state.Revision++
	if err := store.Commit(expected, state, wrapper); err != nil {
		t.Fatal(err)
	}
	tampered := keyStateBytes(t, state)
	if _, err := OpenKeyService(store, scope, workspace, true); !errors.Is(err, keyring.ErrAuthenticationFailed) {
		t.Fatalf("tampered wrap = %v", err)
	}
	state, _, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(tampered, keyStateBytes(t, state)) {
		t.Fatal("tampered wrap reset")
	}
}

func TestBrowserKeyCommitFailuresAndCAS(t *testing.T) {
	scope, workspace := setupBrowserKeys(t)
	store, service := openBrowserKeys(t, scope, workspace, false)
	state, device, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	next := state.Clone()
	next.Revision++
	if err := store.Commit(state.Revision, next, device); err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(state.Revision, next, device); !errors.Is(err, ErrKeyStateChanged) {
		t.Fatalf("CAS lost update = %v", err)
	}
	if _, err := service.Seal("record", nil); !errors.Is(err, ErrKeyStateChanged) {
		t.Fatalf("stale service = %v", err)
	}
	fake := js.Global().Get("__fakeIDB")
	if !fake.Truthy() {
		t.Skip("failure injection requires localdb IndexedDB shim")
	}
	_, service = openBrowserKeys(t, scope, workspace, true)
	before := keyStateBytes(t, service.State())
	for _, failure := range []struct{ method, name string }{{"failNextWrite", "QuotaExceededError"}, {"failNextTransaction", "UnknownError"}} {
		t.Run(failure.method, func(t *testing.T) {
			fake.Call(failure.method, failure.name)
			// AddRecovery performs a readonly check first. For transaction-start
			// injection that check must itself fail, with no acknowledged write.
			code, err := service.AddRecovery()
			if err == nil || code != "" {
				t.Fatalf("failed transaction acknowledged: %q, %v", code, err)
			}
			state, _, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, keyStateBytes(t, state)) || !bytes.Equal(before, keyStateBytes(t, service.State())) {
				t.Fatal("failed write published state")
			}
		})
	}
}

func TestBrowserKeyConcurrentFirstUse(t *testing.T) {
	scope, workspace := setupBrowserKeys(t)
	// Independent connections simulate two tabs. Either a concurrent creator
	// wins or a later opener adopts that same key; neither may overwrite it.
	a, err := NewBrowserKeyStore(scope, workspace)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := NewBrowserKeyStore(scope, workspace)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	type result struct {
		service *KeyService
		err     error
	}
	done := make(chan result, 2)
	for _, store := range []*BrowserKeyStore{a, b} {
		go func() { s, err := OpenKeyService(store, scope, workspace, false); done <- result{s, err} }()
	}
	var winner *KeyService
	for range 2 {
		r := <-done
		if r.err != nil {
			if !errors.Is(r.err, ErrKeyStateChanged) {
				t.Fatal(r.err)
			}
			continue
		}
		t.Cleanup(r.service.Lock)
		if winner != nil && winner.State().Ring.Current().KeyID != r.service.State().Ring.Current().KeyID {
			t.Fatal("two creators acknowledged different WDKs")
		}
		winner = r.service
	}
	if winner == nil {
		t.Fatal("no successful creator")
	}
	envelope, err := winner.Seal("cashflux:dataset", []byte("household secret"))
	if err != nil {
		t.Fatal(err)
	}
	_, reloaded := openBrowserKeys(t, scope, workspace, true)
	assertRecord(t, reloaded, envelope)
}

func TestBrowserKeyCreationWaitsForStrictCommit(t *testing.T) {
	scope, workspace := setupBrowserKeys(t)
	store, err := NewBrowserKeyStore(scope, workspace)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	// Abort AFTER the write request succeeds, before transaction completion.
	// Neither a CryptoKey nor its wrapped WDK may survive or be acknowledged.
	intercept := js.Global().Get("Function").New("db", `
const original = db.transaction;
const result = {strict: false, wrote: false};
db.transaction = function(names, mode, options) {
  const tx = original.call(this, names, mode, options);
  if (mode === 'readwrite') {
    result.strict = options && options.durability === 'strict';
    const objectStore = tx.objectStore.bind(tx);
    tx.objectStore = function(name) {
      const store = objectStore(name), put = store.put.bind(store);
      store.put = function(value, key) {
        const req = put(value, key);
        req.onsuccess = function() { result.wrote = true; tx.abort(); };
        return req;
      };
      return store;
    };
  }
  return tx;
};
result.restore = function() { db.transaction = original; };
return result;
`).Invoke(store.db)
	defer intercept.Call("restore")
	service, err := OpenKeyService(store, scope, workspace, false)
	if err == nil || service != nil {
		t.Fatalf("acknowledged aborted key creation: %v, %v", service, err)
	}
	if !intercept.Get("strict").Bool() || !intercept.Get("wrote").Bool() {
		t.Fatal("test did not reach a strict transaction's successful put")
	}
	state, device, err := store.Load()
	if err != nil || state != nil || device != nil {
		t.Fatalf("aborted key creation persisted data: %v, %v, %v", state, device, err)
	}
	intercept.Call("restore")
	_, service = openBrowserKeys(t, scope, workspace, false)
	if _, err := service.Seal("record", []byte("secret")); err != nil {
		t.Fatal(err)
	}
}

func TestBrowserKeyMetadataValidation(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(js.Value)
	}{
		{"invalid JSON", func(row js.Value) { row.Set("state", "broken") }},
		{"non-string state", func(row js.Value) { row.Set("state", 42) }},
		{"invalid device object", func(row js.Value) {
			row.Set("deviceKey", map[string]any{"algorithm": map[string]any{"name": "AES-GCM"}})
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			scope, workspace := setupBrowserKeys(t)
			store, service := openBrowserKeys(t, scope, workspace, false)
			validState := service.State()
			mutateBrowserKeyRow(t, store, test.mutate)
			if _, err := OpenKeyService(store, scope, workspace, true); !errors.Is(err, ErrInvalidKeyState) {
				t.Fatalf("malformed metadata: %v", err)
			}
			// Even a direct CAS may not replace the malformed metadata.
			validState.Revision++
			if err := store.Commit(validState.Revision-1, validState, service.device); !errors.Is(err, ErrInvalidKeyState) {
				t.Fatalf("CAS replaced malformed metadata: %v", err)
			}
		})
	}

	t.Run("store scope is fixed", func(t *testing.T) {
		scope, workspace := setupBrowserKeys(t)
		store, service := openBrowserKeys(t, scope, workspace, false)
		next := service.State()
		next.Revision++
		next.WorkspaceID = "foreign"
		if err := store.Commit(next.Revision-1, next, service.device); !errors.Is(err, ErrInvalidKeyState) {
			t.Fatalf("foreign state accepted: %v", err)
		}
	})
}
