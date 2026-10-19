// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"bytes"
	"os"
	"syscall/js"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/browserstore"
)

func TestArtifactSaltMigrationPreservesOldAttachmentCiphertext(t *testing.T) {
	if !browserstore.Durable() {
		if !js.Global().Get("crypto").Truthy() || !js.Global().Get("crypto").Get("subtle").Truthy() {
			js.Global().Set("crypto", js.Global().Get("require").Invoke("node:crypto").Get("webcrypto"))
		}
		script, err := os.ReadFile("../localdb/testdata/fake_indexeddb.js")
		if err != nil {
			t.Fatalf("read IndexedDB test shim: %v", err)
		}
		js.Global().Call("eval", string(script))
		browserstore.Init()
	}
	if !browserstore.Durable() {
		t.Fatalf("secure browserstore unavailable: %v", browserstore.InitError())
	}

	const oldSalt = "b2xkLWFydGlmYWN0LXNhbHQ="
	const passcode = "artifact migration test"
	oldPasscode := activePasscode
	oldCache := artifactKeyCache
	activePasscode = passcode
	artifactKeyCache = map[string]js.Value{}
	t.Cleanup(func() {
		activePasscode = oldPasscode
		artifactKeyCache = oldCache
	})

	// Model an attachment already encrypted before the legacy salt is migrated.
	ciphertext, err := encryptArtifactWithSalt([]byte("attachment bytes"), oldSalt)
	if err != nil {
		t.Fatalf("encrypt old attachment: %v", err)
	}

	legacy, restoreLegacy := legacyStorageShim(t)
	defer restoreLegacy()
	previousLegacy := legacy.Call("getItem", artifactSaltKey)
	oldStored, stored, err := browserstore.GetSecure(artifactSaltStoreKey)
	if err != nil {
		t.Fatalf("read prior encrypted salt: %v", err)
	}
	removeDurably(t, artifactSaltStoreKey)
	legacy.Call("setItem", artifactSaltKey, oldSalt)
	t.Cleanup(func() {
		if previousLegacy.IsNull() || previousLegacy.IsUndefined() {
			legacy.Call("removeItem", artifactSaltKey)
		} else {
			legacy.Call("setItem", artifactSaltKey, previousLegacy.String())
		}
		if stored {
			setDurably(t, artifactSaltStoreKey, oldStored)
		} else {
			removeDurably(t, artifactSaltStoreKey)
		}
	})

	migrated, err := artifactSalt()
	if err != nil {
		t.Fatalf("migrate artifact salt: %v", err)
	}
	if migrated != oldSalt {
		t.Fatalf("migrated salt = %q, want original %q", migrated, oldSalt)
	}
	if got := legacy.Call("getItem", artifactSaltKey); !got.IsNull() && !got.IsUndefined() {
		t.Fatalf("legacy salt still present after verified migration: %q", got.String())
	}
	gotSalt, present, err := browserstore.GetSecure(artifactSaltStoreKey)
	if err != nil || !present || gotSalt != oldSalt {
		t.Fatalf("encrypted salt = %q, present %t, error %v; want verified old salt", gotSalt, present, err)
	}

	plain, err := decryptArtifactSync(ciphertext)
	if err != nil {
		t.Fatalf("decrypt attachment after migration: %v", err)
	}
	if !bytes.Equal(plain, []byte("attachment bytes")) {
		t.Fatalf("decrypted attachment = %q", plain)
	}
}

func legacyStorageShim(t *testing.T) (js.Value, func()) {
	t.Helper()
	global := js.Global()
	previous := global.Get("localStorage")
	values := map[string]string{}
	object := js.Global().Get("Object").New()
	var callbacks []js.Func
	add := func(name string, fn func([]js.Value) any) {
		callback := js.FuncOf(func(_ js.Value, args []js.Value) any { return fn(args) })
		callbacks = append(callbacks, callback)
		object.Set(name, callback)
	}
	add("getItem", func(args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		if value, ok := values[args[0].String()]; ok {
			return value
		}
		return nil
	})
	add("setItem", func(args []js.Value) any {
		if len(args) > 1 {
			values[args[0].String()] = args[1].String()
		}
		return nil
	})
	add("removeItem", func(args []js.Value) any {
		if len(args) > 0 {
			delete(values, args[0].String())
		}
		return nil
	})
	global.Set("localStorage", object)
	return object, func() {
		global.Set("localStorage", previous)
		for _, callback := range callbacks {
			callback.Release()
		}
	}
}

func setDurably(t *testing.T, key, value string) {
	t.Helper()
	done := make(chan error, 1)
	browserstore.SetSecure(key, value, func(err error) { done <- err })
	waitForStorageResult(t, done)
}

func removeDurably(t *testing.T, key string) {
	t.Helper()
	done := make(chan error, 1)
	browserstore.RemoveSecure(key, func(err error) { done <- err })
	waitForStorageResult(t, done)
}

func waitForStorageResult(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("durable browserstore operation: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for durable browserstore operation")
	}
}
