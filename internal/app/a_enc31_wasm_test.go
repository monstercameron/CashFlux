// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"syscall/js"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/aicred"
	"github.com/monstercameron/CashFlux/internal/applock"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/atrest"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/keyring"
)

func enc31Wait(t *testing.T, start func(func(error))) error {
	t.Helper()
	done := make(chan error, 1)
	start(func(err error) { done <- err })
	select {
	case err := <-done:
		return err
	case <-time.After(30 * time.Second):
		t.Fatal("protected operation timed out")
		return nil
	}
}

func enc31RawRecord(t *testing.T, key, raw string) {
	t.Helper()
	req := js.Global().Get("indexedDB").Call("open", "cashflux-kv", 1)
	opened := make(chan struct{}, 1)
	upgrade := js.FuncOf(func(js.Value, []js.Value) any { req.Get("result").Call("createObjectStore", "kv"); return nil })
	ok := js.FuncOf(func(js.Value, []js.Value) any { opened <- struct{}{}; return nil })
	req.Set("onupgradeneeded", upgrade)
	req.Set("onsuccess", ok)
	<-opened
	db := req.Get("result")
	tx := db.Call("transaction", "kv", "readwrite", map[string]any{"durability": "strict"})
	done := make(chan struct{}, 1)
	complete := js.FuncOf(func(js.Value, []js.Value) any { done <- struct{}{}; return nil })
	tx.Set("oncomplete", complete)
	tx.Call("objectStore", "kv").Call("put", raw, key)
	<-done
	complete.Release()
	upgrade.Release()
	ok.Release()
	db.Call("close")
}

func enc31FakeDocument(t *testing.T) {
	t.Helper()
	old := js.Global().Get("document")
	js.Global().Call("eval", `(function(){
      function element(tag) {
        return {tagName:tag, id:"", textContent:"", value:"", disabled:false, style:{display:""}, children:[], attrs:{}, events:{},
          setAttribute:function(k,v){this.attrs[k]=v;},
          appendChild:function(e){e.parent=this;this.children.push(e);},
          addEventListener:function(k,fn){this.events[k]=fn;},
          remove:function(){if(this.parent)this.parent.children=this.parent.children.filter(e=>e!==this);},
          focus:function(){document.activeElement=this;},
          click:function(){if(!this.disabled && this.events.click)this.events.click({});}
        };
      }
      var body=element("body");
      globalThis.document={body:body, activeElement:body, createElement:element, querySelector:function(){return null;},
        getElementById:function(id){function find(e){if(e.id===id)return e;for(var c of e.children){var got=find(c);if(got)return got;}return null;}return find(body);}
      };
      var root=element("div");root.id="app";body.appendChild(root);
    })()`)
	t.Cleanup(func() { js.Global().Set("document", old) })
}

// This fixture initializes the shared Node page once: browserstore intentionally
// has no reset/scope-swap API. All disk/crypto operations use the Node-only shim;
// no real browser profile or user's stored records are opened by this fixture.
func TestENC31ProtectedBootAndAppLockLifecycle(t *testing.T) {
	if !js.Global().Get("process").Truthy() {
		t.Skip("isolated Node IndexedDB fixture only")
	}
	browserstore.Barrier()
	oldBroadcast, oldLS := js.Global().Get("BroadcastChannel"), js.Global().Get("localStorage")
	oldApp, oldPasscode, oldPending := appstate.Default, activePasscode, pendingEnvelopeRaw
	oldHydrated, oldLocal, oldSuspended := hydrationSucceeded, hadLocalDataset, suspendAutosave
	oldRecovered := bootOpenedAfterProtectedError
	js.Global().Set("crypto", js.Global().Get("require").Invoke("node:crypto").Get("webcrypto"))
	js.Global().Set("BroadcastChannel", js.Undefined())
	js.Global().Set("localStorage", js.Undefined())
	script, err := os.ReadFile("../localdb/testdata/fake_indexeddb.js")
	if err != nil {
		t.Fatal(err)
	}
	js.Global().Call("eval", string(script))
	enc31FakeDocument(t)
	t.Cleanup(func() {
		browserstore.Barrier()
		appstate.Default, activePasscode, pendingEnvelopeRaw = oldApp, oldPasscode, oldPending
		hydrationSucceeded, hadLocalDataset, suspendAutosave = oldHydrated, oldLocal, oldSuspended
		bootOpenedAfterProtectedError = oldRecovered
		js.Global().Set("BroadcastChannel", oldBroadcast)
		js.Global().Set("localStorage", oldLS)
	})
	const oldPass, newPass = "enc31-old-passcode", "enc31-new-passcode"
	source, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := source.PutAccount(domain.Account{ID: "enc31-account", Name: "Protected household", Type: domain.TypeChecking, Class: domain.ClassAsset, Currency: "USD", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}); err != nil {
		t.Fatal(err)
	}
	dataset, err := source.ExportJSONRedacted()
	if err != nil {
		t.Fatal(err)
	}
	keys, err := atrest.NewBrowserKeyStore(keyring.LocalScope(""), "legacy-browser-profile")
	if err != nil {
		t.Fatal(err)
	}
	service, err := atrest.OpenKeyService(keys, keyring.LocalScope(""), "legacy-browser-profile", false)
	if err != nil {
		t.Fatal(err)
	}
	recovery, err := service.AddRecovery()
	if err != nil {
		t.Fatal(err)
	}
	config := applock.Config{}.WithPasscode(oldPass, "enc31-salt", 0, "")
	configJSON, _ := json.Marshal(config)
	for key, value := range map[string]string{datasetStoreKey: string(dataset), appLockKey: string(configJSON)} {
		env, err := service.Seal(key, []byte(value))
		if err != nil {
			t.Fatal(err)
		}
		raw, err := env.Encode()
		if err != nil {
			t.Fatal(err)
		}
		enc31RawRecord(t, key, string(raw))
	}
	if err := service.EnableAppLock(oldPass); err != nil {
		t.Fatal(err)
	}
	service.Lock()
	keys.Close()
	browserstore.Init()
	if !errors.Is(browserstore.InitError(), atrest.ErrLocked) {
		t.Fatalf("InitError = %v, want locked", browserstore.InitError())
	}
	appstate.Default, err = appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	activePasscode, pendingEnvelopeRaw = "", ""
	hydrationSucceeded, hadLocalDataset, suspendAutosave = false, false, false
	doc := js.Global().Get("document")

	t.Run("InitError gates every protected boot action and never seeds", func(t *testing.T) {
		before := js.Global().Get("__fakeIDB").Call("dump", "cashflux-kv").String()
		continued := false
		resumeProtectedBoot(browserstore.InitError(), func(cause error, resume func()) { showProtectedStorageGate(cause, resume) }, func() { continued = true; hydrateDataset() })
		if continued || len(appstate.Default.Accounts()) != 0 || hydrationSucceeded {
			t.Fatal("locked boot hydrated or seeded")
		}
		if !appLockActive() || !doc.Call("getElementById", "app").Get("inert").Bool() {
			t.Fatal("boot did not cover and disable the app")
		}
		gate := doc.Call("getElementById", appLockGateID)
		if !gate.Truthy() || doc.Call("getElementById", "cf-protected-reset").Truthy() {
			t.Fatal("destructive gate action")
		}
		if doc.Call("getElementById", "cf-protected-repair").Get("style").Get("display").String() != "none" {
			t.Fatal("device repair offered before explicit unlock")
		}
		doc.Call("getElementById", "cf-applock-input").Set("value", "wrong-passcode")
		doc.Call("getElementById", "cf-protected-unlock").Call("click")
		browserstore.Barrier()
		if continued || len(appstate.Default.Accounts()) != 0 {
			t.Fatal("wrong unlock bypassed boot gate")
		}
		if after := js.Global().Get("__fakeIDB").Call("dump", "cashflux-kv").String(); after != before {
			t.Fatal("failed boot/unlock changed protected records")
		}
		doc.Call("getElementById", "cf-applock-input").Set("value", oldPass)
		doc.Call("getElementById", "cf-protected-unlock").Call("click")
		deadline := time.Now().Add(20 * time.Second)
		for (!continued || !hydrationSucceeded || len(appstate.Default.Accounts()) != 1 || appLockActive()) && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
		if !continued || !hydrationSucceeded || len(appstate.Default.Accounts()) != 1 || appLockActive() {
			t.Fatal("successful unlock did not resume hydration")
		}
	})
	t.Run("all boot errors wait for explicit continuation", func(t *testing.T) {
		for _, cause := range []error{atrest.ErrRecoveryRequired, atrest.ErrMissingDeviceKey, atrest.ErrInvalidEnvelope, &browserstore.RecordError{Key: datasetStoreKey, Err: atrest.ErrLocked}, errProtectedStorageUnavailable} {
			var resume func()
			reads := 0
			resumeProtectedBoot(cause, func(_ error, next func()) { resume = next }, func() { reads++ })
			if reads != 0 || resume == nil {
				t.Fatalf("protected reads escaped gate for %v", cause)
			}
			resume()
			if reads != 1 {
				t.Fatal("continuation did not resume")
			}
		}
	})
	t.Run("lock waits and unlock restores authenticated records before hydration", func(t *testing.T) {
		showAppLockGate()
		deadline := time.Now().Add(20 * time.Second)
		for appLockClosing && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
		browserstore.Barrier()
		if activePasscode != "" || !appLockActive() || !suspendAutosave {
			t.Fatal("lock did not finish before offering unlock")
		}
		if _, _, err := browserstore.GetSecure(datasetStoreKey); !errors.Is(err, atrest.ErrLocked) {
			t.Fatalf("locked read = %v", err)
		}
		if err := enc31Wait(t, func(done func(error)) { unlockProtectedSession(oldPass, false, done) }); err != nil {
			t.Fatal(err)
		}
		appstate.Default, err = appstate.New(&bytes.Buffer{}, false)
		if err != nil {
			t.Fatal(err)
		}
		hydrateDataset()
		if !hydrationSucceeded || len(appstate.Default.Accounts()) != 1 {
			t.Fatal("unlock could not hydrate original data")
		}
		unlockGate(doc, doc.Call("getElementById", appLockGateID))
		suspendAutosave = false
	})
	t.Run("passkey removal acknowledges only an atomic durable commit", func(t *testing.T) {
		values := map[string]string{webauthnCredKey: "credential-id", webauthnSaltKey: "prf-salt", webauthnVaultKey: "prf-envelope"}
		if err := enc31Wait(t, func(done func(error)) { browserstore.SetSecureBatch(values, nil, done) }); err != nil {
			t.Fatal(err)
		}
		rememberPasskeyForLock()
		js.Global().Get("__fakeIDB").Call("failNextWrite", "QuotaExceededError")
		if err := enc31Wait(t, clearPasskeyThen); err == nil {
			t.Fatal("failed passkey removal acknowledged")
		}
		if !hasPasskey() || lockedPasskey[0] != "credential-id" {
			t.Fatal("failed removal dropped session metadata")
		}
		if err := enc31Wait(t, clearPasskeyThen); err != nil {
			t.Fatal(err)
		}
		if hasPasskey() || lockedPasskey != [3]string{} {
			t.Fatal("successful removal retained stale PRF state")
		}
	})
	t.Run("corrupt vault blocks every passcode and record change", func(t *testing.T) {
		raw := browserstore.GetString(credVaultStoreKey)
		if err := enc31Wait(t, func(done func(error)) { browserstore.SetSecure(credVaultStoreKey, "broken-vault", done) }); err != nil {
			t.Fatal(err)
		}
		revision := browserstore.SecureKeyState().Revision
		before := js.Global().Get("__fakeIDB").Call("dump", "cashflux-kv").String()
		err := runSecureLockTransition(newPass, loadAppLock().WithPasscode(newPass, "bad-vault-salt", 0, ""), false)
		if !errors.Is(err, errCredVaultUnreadable) {
			t.Fatalf("corrupt vault = %v", err)
		}
		if browserstore.SecureKeyState().Revision != revision || browserstore.GetString(secureLockTransitionKey) != "" {
			t.Fatal("failed preparation changed the wrap or wrote a journal")
		}
		if js.Global().Get("__fakeIDB").Call("dump", "cashflux-kv").String() != before {
			t.Fatal("corrupt vault was overwritten")
		}
		if err := enc31Wait(t, func(done func(error)) { browserstore.SetSecure(credVaultStoreKey, raw, done) }); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("passcode change preserves the legacy bank vault and inner dataset", func(t *testing.T) {
		vaultJSON, _ := json.Marshal(credVault{"enc31-account": {Username: "bank-user", Password: "bank-secret"}})
		encrypted, err := encryptDatasetSync(vaultJSON, oldPass)
		if err != nil {
			t.Fatal(err)
		}
		if err := enc31Wait(t, func(done func(error)) {
			browserstore.SetSecure(credVaultStoreKey, base64.StdEncoding.EncodeToString(encrypted), done)
		}); err != nil {
			t.Fatal(err)
		}
		inner, err := encryptDatasetSync(dataset, oldPass)
		if err != nil {
			t.Fatal(err)
		}
		if err := enc31Wait(t, func(done func(error)) { browserstore.SetSecure(datasetStoreKey, string(inner), done) }); err != nil {
			t.Fatal(err)
		}
		aiEncrypted, err := encryptDatasetSync([]byte("sk-enc31-device-credential"), oldPass)
		if err != nil {
			t.Fatal(err)
		}
		aiRecord, err := aicred.New("", string(aiEncrypted), true)
		if err != nil {
			t.Fatal(err)
		}
		aiRaw, err := aicred.Marshal(aiRecord)
		if err != nil {
			t.Fatal(err)
		}
		if err := enc31Wait(t, func(done func(error)) { browserstore.SetSecure(aiCredentialStoreKey, string(aiRaw), done) }); err != nil {
			t.Fatal(err)
		}
		next := loadAppLock().WithPasscode(newPass, "enc31-new-salt", 0, "")
		if err := runSecureLockTransition(newPass, next, false); err != nil {
			t.Fatal(err)
		}
		migratedAI, err := aicred.Parse([]byte(browserstore.GetString(aiCredentialStoreKey)))
		if err != nil {
			t.Fatal(err)
		}
		aiKey, err := decryptEnvelopeSync([]byte(migratedAI.Envelope), newPass)
		if err != nil || string(aiKey) != "sk-enc31-device-credential" {
			t.Fatalf("migrated AI credential = %q, %v", aiKey, err)
		}
		got, err := awaitVaultTest(t, func(done func(credVault, error)) { loadCredVaultWithPasscode(newPass, done) })
		if err != nil || got["enc31-account"].Password != "bank-secret" {
			t.Fatalf("changed-passcode vault = %#v, %v", got, err)
		}
		if err := enc31Wait(t, func(done func(error)) { browserstore.LockSecureThen(func() { done(nil) }) }); err != nil {
			t.Fatal(err)
		}
		if err := enc31Wait(t, func(done func(error)) { browserstore.UnlockSecureThen(oldPass, done) }); err == nil {
			t.Fatal("old passcode still opens WDK")
		}
		if err := enc31Wait(t, func(done func(error)) { unlockProtectedSession(newPass, false, done) }); err != nil {
			t.Fatal(err)
		}
		raw, _, err := browserstore.GetSecure(datasetStoreKey)
		if err != nil {
			t.Fatal(err)
		}
		plain, err := decryptEnvelopeSync([]byte(raw), newPass)
		if err != nil || !bytes.Equal(plain, dataset) {
			t.Fatalf("inner dataset after change = %v", err)
		}
	})
	t.Run("interrupted rewrap resumes without orphaning the vault", func(t *testing.T) {
		target := loadAppLock().WithPasscode(oldPass, "enc31-resume-salt", 0, "")
		prepared, err := prepareSecureLockTransition(oldPass, target, false)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(prepared)
		if err := enc31Wait(t, func(done func(error)) { browserstore.SetSecure(secureLockTransitionKey, string(raw), done) }); err != nil {
			t.Fatal(err)
		}
		if err := enc31Wait(t, func(done func(error)) { browserstore.ChangePasscodeSecure(oldPass, done) }); err != nil {
			t.Fatal(err)
		}
		browserstore.LockSecureThen(nil)
		browserstore.Barrier()
		activePasscode = ""
		if err := enc31Wait(t, func(done func(error)) { unlockProtectedSession(oldPass, false, done) }); err != nil {
			t.Fatal(err)
		}
		vault, err := awaitVaultTest(t, func(done func(credVault, error)) { loadCredVaultWithPasscode(oldPass, done) })
		if err != nil || vault["enc31-account"].Password != "bank-secret" {
			t.Fatalf("resumed vault = %#v, %v", vault, err)
		}
		if browserstore.GetString(secureLockTransitionKey) != "" {
			t.Fatal("committed transition left secret journal")
		}
	})
	t.Run("failed wrap commit keeps records and resumable journal", func(t *testing.T) {
		prepared, err := prepareSecureLockTransition(newPass, loadAppLock().WithPasscode(newPass, "failed-wrap-salt", 0, ""), false)
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(prepared)
		journal := string(encoded)
		if err := enc31Wait(t, func(done func(error)) { browserstore.SetSecure(secureLockTransitionKey, journal, done) }); err != nil {
			t.Fatal(err)
		}
		before := browserstore.GetString(credVaultStoreKey)
		revision := browserstore.SecureKeyState().Revision
		js.Global().Get("__fakeIDB").Call("failNextWrite", "QuotaExceededError")
		if err := commitSecureLockTransition(prepared, journal); err == nil {
			t.Fatal("failed key commit acknowledged")
		}
		if browserstore.SecureKeyState().Revision != revision || browserstore.GetString(credVaultStoreKey) != before || browserstore.GetString(secureLockTransitionKey) != journal {
			t.Fatal("failed wrap commit changed records or dropped recovery journal")
		}
		passcode, changed, err := recoverSecureLockTransition()
		if err != nil || !changed || passcode != newPass {
			t.Fatalf("retry pending change = %v, %v", changed, err)
		}
		vault, err := awaitVaultTest(t, func(done func(credVault, error)) { loadCredVaultWithPasscode(newPass, done) })
		if err != nil || vault["enc31-account"].Password != "bank-secret" {
			t.Fatalf("retried vault = %#v, %v", vault, err)
		}
	})
	t.Run("concurrent record edits are preserved during a rewrap", func(t *testing.T) {
		target := loadAppLock().WithPasscode(newPass, "conditional-salt", 0, "")
		prepared, err := prepareSecureLockTransition(newPass, target, false)
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(prepared)
		journal := string(encoded)
		if err := enc31Wait(t, func(done func(error)) { browserstore.SetSecure(secureLockTransitionKey, journal, done) }); err != nil {
			t.Fatal(err)
		}
		original := prepared.Expected[appLockKey]
		edited := loadAppLock()
		edited.Hint = "changed in another tab"
		editedRaw, _ := json.Marshal(edited)
		if err := enc31Wait(t, func(done func(error)) { browserstore.SetSecure(appLockKey, string(editedRaw), done) }); err != nil {
			t.Fatal(err)
		}
		if err := commitSecureLockTransition(prepared, journal); !errors.Is(err, atrest.ErrRecordChanged) {
			t.Fatalf("concurrent edit = %v", err)
		}
		if browserstore.GetString(appLockKey) != string(editedRaw) || browserstore.GetString(secureLockTransitionKey) != journal {
			t.Fatal("concurrent edit was overwritten or journal lost")
		}
		// Explicit fixture repair allows the suspended transition to be retried.
		if err := enc31Wait(t, func(done func(error)) { browserstore.SetSecure(appLockKey, original, done) }); err != nil {
			t.Fatal(err)
		}
		if _, changed, err := recoverSecureLockTransition(); err != nil || !changed {
			t.Fatalf("retry after explicit fixture repair = %v, %v", changed, err)
		}
	})
	t.Run("recovery unlock precedes protected reads", func(t *testing.T) {
		browserstore.LockSecureThen(nil)
		browserstore.Barrier()
		activePasscode = ""
		if err := enc31Wait(t, func(done func(error)) { unlockProtectedSession(recovery, true, done) }); err != nil {
			t.Fatal(err)
		}
		if activePasscode != "" {
			t.Fatal("recovery key became transport passcode")
		}
		if _, have, err := browserstore.GetSecure(datasetStoreKey); err != nil || !have {
			t.Fatalf("recovery read = %v, %v", have, err)
		}
		activePasscode = newPass // explicit legacy passcode for the inner v1 envelope
	})
	t.Run("device repair is offered only after explicit unlock", func(t *testing.T) {
		ready := false
		revision := browserstore.SecureKeyState().Revision
		showProtectedStorageGate(atrest.ErrMissingDeviceKey, func() { ready = true })
		repair := doc.Call("getElementById", "cf-protected-repair")
		if repair.Get("style").Get("display").String() != "none" {
			t.Fatal("repair offered before unlock")
		}
		doc.Call("getElementById", "cf-applock-input").Set("value", newPass)
		doc.Call("getElementById", "cf-protected-unlock").Call("click")
		deadline := time.Now().Add(20 * time.Second)
		for repair.Get("style").Get("display").String() == "none" && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
		if ready || repair.Get("style").Get("display").String() != "block" || browserstore.SecureKeyState().Revision != revision {
			t.Fatal("unlock repaired automatically or bypassed the explicit choice")
		}
		doc.Call("getElementById", "cf-protected-unlock").Call("click")
		if !ready {
			t.Fatal("continue without repair did not resume")
		}
	})
	t.Run("disable keeps durable household readable and encrypted", func(t *testing.T) {
		if err := runSecureLockTransition("", applock.Config{}, true); err != nil {
			t.Fatal(err)
		}
		if browserstore.SecureKeyState().AppLock || activePasscode != "" {
			t.Fatal("App Lock was not disabled")
		}
		raw, have, err := browserstore.GetSecure(datasetStoreKey)
		if err != nil || !have || !bytes.Equal([]byte(raw), dataset) {
			t.Fatalf("disabled dataset readable = %v, %v", have, err)
		}
		keyStore, err := atrest.NewBrowserKeyStore(keyring.LocalScope(""), "legacy-browser-profile")
		if err != nil {
			t.Fatal(err)
		}
		defer keyStore.Close()
		reopened, err := atrest.OpenKeyService(keyStore, keyring.LocalScope(""), "legacy-browser-profile", true)
		if err != nil {
			t.Fatal(err)
		}
		defer reopened.Lock()
		if _, err := reopened.Seal("cashflux:test", []byte("readable after reload")); err != nil {
			t.Fatal(err)
		}
		dump := js.Global().Get("__fakeIDB").Call("dump", "cashflux-kv").String()
		for _, secret := range []string{"Protected household", "bank-secret", "sk-enc31-device-credential", oldPass, newPass} {
			if strings.Contains(dump, secret) {
				t.Fatalf("plaintext persisted: %q", secret)
			}
		}
	})
	t.Run("legacy UI-only App Lock upgrades before boot can continue", func(t *testing.T) {
		saveAppLock(applock.Config{}.WithPasscode(oldPass, "legacy-upgrade-salt", 0, ""))
		browserstore.Barrier()
		if browserstore.SecureKeyState().AppLock {
			t.Fatal("fixture is already secure-locked")
		}
		resumed := make(chan struct{}, 1)
		go func() { awaitProtectedStorageBoot(); resumed <- struct{}{} }()
		deadline := time.Now().Add(20 * time.Second)
		for !appLockActive() && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
		if !appLockActive() {
			t.Fatal("legacy lock boot did not gate")
		}
		select {
		case <-resumed:
			t.Fatal("legacy boot continued without passcode")
		default:
		}
		input := doc.Call("getElementById", "cf-applock-input")
		input.Set("value", oldPass)
		doc.Call("getElementById", "cf-protected-unlock").Call("click")
		select {
		case <-resumed:
		case <-time.After(20 * time.Second):
			t.Fatal("legacy boot unlock timed out")
		}
		if !browserstore.SecureKeyState().AppLock || activePasscode != oldPass {
			t.Fatal("legacy boot continued before secure enable")
		}
		if err := runSecureLockTransition("", applock.Config{}, true); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("enable and disable UI callbacks wait for secure lifecycle", func(t *testing.T) {
		previousDone := appLockOnDone
		defer func() { appLockOnDone = previousDone }()
		enabled := make(chan error, 1)
		appLockOnDone = func() {
			if !browserstore.SecureKeyState().AppLock || activePasscode != newPass {
				enabled <- errors.New("enable callback preceded durable lifecycle")
				return
			}
			enabled <- nil
		}
		if enableAppLock(newPass, 0, "") {
			t.Fatal("async enable acknowledged synchronously")
		}
		select {
		case err := <-enabled:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(30 * time.Second):
			t.Fatal("enable callback timed out")
		}
		if err := enc31Wait(t, func(done func(error)) {
			disableAppLock(func() {
				if browserstore.SecureKeyState().AppLock || activePasscode != "" {
					done(errors.New("disable callback preceded durable lifecycle"))
					return
				}
				done(nil)
			})
		}); err != nil {
			t.Fatal(err)
		}
		if raw, have, err := browserstore.GetSecure(datasetStoreKey); err != nil || !have || raw != string(dataset) {
			t.Fatalf("UI disable lost data: %v", err)
		}
	})

	t.Run("empty protected recovery never seeds sample data", func(t *testing.T) {
		if err := enc31Wait(t, func(done func(error)) { browserstore.RemoveSecure(datasetStoreKey, done) }); err != nil {
			t.Fatal(err)
		}
		previous := appstate.Default
		defer func() { appstate.Default = previous }()
		appstate.Default, err = appstate.New(&bytes.Buffer{}, false)
		if err != nil {
			t.Fatal(err)
		}
		bootOpenedAfterProtectedError = true
		hydrateOpenedProtectedStorage()
		if len(appstate.Default.Accounts()) != 0 || !hydrationSucceeded || hadLocalDataset {
			t.Fatal("empty recovered profile was replaced with sample data")
		}
		if _, have, err := browserstore.GetSecure(datasetStoreKey); err != nil || have {
			t.Fatalf("empty recovery wrote dataset: %v, %v", have, err)
		}
		if err := enc31Wait(t, func(done func(error)) { browserstore.SetSecure(datasetStoreKey, string(dataset), done) }); err != nil {
			t.Fatal(err)
		}
	})

}
