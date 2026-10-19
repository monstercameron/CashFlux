// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"syscall/js"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/cryptobox"
)

func awaitVaultTest[T any](t *testing.T, start func(func(T, error))) (T, error) {
	t.Helper()
	type result struct {
		value T
		err   error
	}
	ch := make(chan result, 1)
	start(func(value T, err error) { ch <- result{value, err} })
	select {
	case got := <-ch:
		return got.value, got.err
	case <-time.After(5 * time.Second):
		var zero T
		t.Fatal("timed out waiting for Web Crypto callback")
		return zero, nil
	}
}

func requireVaultWebCrypto(t *testing.T) {
	t.Helper()
	// go test's wasm runner uses Node and exposes crypto.subtle, but its
	// synchronous test harness does not pump the Promise callbacks needed by
	// Web Crypto while a test is running. Keep these integration checks for a
	// browser runner; all other app wasm tests remain enabled in Node.
	process := js.Global().Get("process")
	if !process.IsUndefined() && !process.IsNull() {
		t.Skip("Web Crypto async callbacks are unavailable in the Node wasm test runner")
	}
	crypto := js.Global().Get("crypto")
	if crypto.IsUndefined() || crypto.IsNull() {
		t.Skip("Web Crypto unavailable: crypto.subtle is missing in the wasm test runner")
	}
	subtle := crypto.Get("subtle")
	if subtle.IsUndefined() || subtle.IsNull() {
		t.Skip("Web Crypto unavailable: crypto.subtle is missing in the wasm test runner")
	}
}

func TestSV07VaultRekeysFromOldPasscodeToNewPasscode(t *testing.T) {
	requireVaultWebCrypto(t)
	plain, err := json.Marshal(credVault{"account-1": {Username: "cam", Password: "secret"}})
	if err != nil {
		t.Fatal(err)
	}
	env, err := awaitVaultTest(t, func(done func([]byte, error)) { encryptDataset(plain, "passcode-A", done) })
	if err != nil {
		t.Fatalf("encrypt under old passcode: %v", err)
	}
	browserstore.Set(credVaultStoreKey, base64.StdEncoding.EncodeToString(env))
	t.Cleanup(func() { browserstore.Remove(credVaultStoreKey) })

	_, err = awaitVaultTest(t, func(done func(struct{}, error)) {
		rekeyCredVault("passcode-A", "passcode-B", func(err error) { done(struct{}{}, err) })
	})
	if err != nil {
		t.Fatalf("rekey vault: %v", err)
	}
	gotRaw := browserstore.GetString(credVaultStoreKey)
	gotEnv, err := base64.StdEncoding.DecodeString(gotRaw)
	if err != nil {
		t.Fatalf("decode rekeyed vault: %v", err)
	}
	vault, err := awaitVaultTest(t, func(done func(credVault, error)) {
		env, ok := cryptobox.Parse(gotEnv)
		if !ok {
			done(nil, errors.New("rekeyed envelope did not parse"))
			return
		}
		decryptDataset(env, "passcode-B", func(plain []byte, err error) {
			if err != nil {
				done(nil, err)
				return
			}
			var v credVault
			err = json.Unmarshal(plain, &v)
			done(v, err)
		})
	})
	if err != nil {
		t.Fatalf("open with new passcode: %v", err)
	}
	if got := vault["account-1"]; got.Username != "cam" || got.Password != "secret" {
		t.Fatalf("rekeyed credential = %+v", got)
	}
}

func TestSV07CorruptVaultRemainsByteIdenticalAfterFailedRekey(t *testing.T) {
	const corrupt = "not-base64-vault-bytes-unchanged"
	browserstore.Set(credVaultStoreKey, corrupt)
	t.Cleanup(func() { browserstore.Remove(credVaultStoreKey) })

	_, err := awaitVaultTest(t, func(done func(struct{}, error)) {
		rekeyCredVault("passcode-A", "passcode-B", func(err error) { done(struct{}{}, err) })
	})
	if err == nil {
		t.Fatal("rekeying corrupt vault unexpectedly succeeded")
	}
	if got := browserstore.GetString(credVaultStoreKey); got != corrupt {
		t.Fatalf("corrupt vault changed: got %q, want original bytes %q", got, corrupt)
	}
}
