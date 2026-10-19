// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/applock"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/browserstore"
)

func TestSVR2UnlockQuarantinesOnlyAfterPasscodeVerification(t *testing.T) {
	requireVaultWebCrypto(t)
	oldApp := appstate.Default
	oldLock := browserstore.GetString(appLockKey)
	oldPasscode := activePasscode
	oldPending := pendingEnvelopeRaw
	oldQuarantined, oldQuarantinePending := datasetQuarantined, quarantinePending
	oldQuarantineRaw, oldHydrated, oldHadLocal := quarantineRaw, hydrationSucceeded, hadLocalDataset
	oldEnvelopeKeys := make(map[string]bool)
	for _, key := range browserstore.Keys() {
		if len(key) >= len(quarantinePrefix) && key[:len(quarantinePrefix)] == quarantinePrefix {
			oldEnvelopeKeys[key] = true
		}
	}
	t.Cleanup(func() {
		appstate.Default = oldApp
		activePasscode = oldPasscode
		pendingEnvelopeRaw = oldPending
		datasetQuarantined, quarantinePending = oldQuarantined, oldQuarantinePending
		quarantineRaw, hydrationSucceeded, hadLocalDataset = oldQuarantineRaw, oldHydrated, oldHadLocal
		if oldLock == "" {
			browserstore.Remove(appLockKey)
		} else {
			browserstore.Set(appLockKey, oldLock)
		}
		for _, key := range browserstore.Keys() {
			if len(key) >= len(quarantinePrefix) && key[:len(quarantinePrefix)] == quarantinePrefix && !oldEnvelopeKeys[key] {
				browserstore.Remove(key)
			}
		}
	})

	if err := appstate.Init(nil, false); err != nil {
		t.Fatalf("initialize empty app: %v", err)
	}
	saveAppLock(applock.Config{}.WithPasscode("correct", "svr2-test-salt", 0, ""))
	plain := []byte("this is not a CashFlux dataset")
	envelope, err := awaitVaultTest(t, func(done func([]byte, error)) {
		encryptDataset(plain, "correct", done)
	})
	if err != nil {
		t.Fatalf("encrypt corrupt dataset plaintext: %v", err)
	}

	// A direct wrong-passcode attempt must leave the encrypted bytes pending and
	// must not start quarantine.
	pendingEnvelopeRaw = string(envelope)
	datasetQuarantined, quarantinePending, quarantineRaw = false, false, ""
	hydrationSucceeded, hadLocalDataset = false, true
	hydrateFromPasscode("wrong")
	if pendingEnvelopeRaw != string(envelope) {
		t.Fatal("wrong passcode cleared the pending envelope")
	}
	if datasetQuarantined || quarantinePending || quarantineRaw != "" {
		t.Fatal("wrong passcode started quarantining the dataset")
	}
	if len(appstate.Default.Accounts()) != 0 {
		t.Fatal("wrong passcode unexpectedly imported or seeded data")
	}

	// The stored passcode is valid, so successful decryption followed by an
	// ImportJSON failure must preserve the original envelope in quarantine.
	hydrateFromPasscode("correct")
	deadline := time.Now().Add(10 * time.Second)
	for !datasetQuarantined && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if pendingEnvelopeRaw != "" {
		t.Fatal("verified corrupt dataset remained pending instead of being quarantined")
	}
	if quarantineRaw != string(envelope) {
		t.Fatal("quarantine did not retain the exact encrypted envelope")
	}
	if !datasetQuarantined {
		t.Fatal("verified passcode plus corrupt plaintext did not start quarantine")
	}
	var reason string
	for _, key := range browserstore.Keys() {
		if len(key) > len(quarantinePrefix)+len(":reason") && key[:len(quarantinePrefix)] == quarantinePrefix && key[len(key)-len(":reason"):] == ":reason" {
			reason = browserstore.GetString(key)
		}
	}
	if reason == "" {
		t.Fatal("quarantine did not save a reason for the corrupt decrypted dataset")
	}
	if hydrationSucceeded {
		t.Fatal("failed import marked hydration successful")
	}
	if len(appstate.Default.Accounts()) != 0 {
		t.Fatal("corrupt plaintext unexpectedly imported or seeded sample data")
	}
}
