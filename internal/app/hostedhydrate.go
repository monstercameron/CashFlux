// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strings"
	"sync"

	"github.com/monstercameron/GoWebComponents/v5/state"
)

const hostedHydrationRevAtomID = "hosted:hydration:rev"

type hostedHydrationPhase string

const (
	hostedHydrationReady     hostedHydrationPhase = "ready"
	hostedHydrationWaiting   hostedHydrationPhase = "waiting"
	hostedHydrationLoading   hostedHydrationPhase = "loading"
	hostedHydrationLocked    hostedHydrationPhase = "locked"
	hostedHydrationVerifying hostedHydrationPhase = "verifying"
	hostedHydrationApplying  hostedHydrationPhase = "applying"
	hostedHydrationError     hostedHydrationPhase = "error"
)

type hostedHydrationStatus struct {
	Required          bool
	Phase             hostedHydrationPhase
	Message           string
	EncryptedSnapshot []byte
}

var hostedHydration = struct {
	sync.Mutex
	status hostedHydrationStatus
}{status: hostedHydrationStatus{Phase: hostedHydrationReady}}

var (
	capturedHostedHydrationRev state.Atom[int]
	hostedHydrationRevCaptured bool
)

func initializeHostedHydration(required bool) {
	phase := hostedHydrationReady
	if required {
		phase = hostedHydrationWaiting
	}
	setHostedHydration(hostedHydrationStatus{Required: required, Phase: phase})
}

func currentHostedHydration() hostedHydrationStatus {
	hostedHydration.Lock()
	defer hostedHydration.Unlock()
	out := hostedHydration.status
	out.EncryptedSnapshot = append([]byte(nil), out.EncryptedSnapshot...)
	return out
}

func hostedHydrationRequired() bool {
	return currentHostedHydration().Required
}

func setHostedHydration(next hostedHydrationStatus) {
	next.EncryptedSnapshot = append([]byte(nil), next.EncryptedSnapshot...)
	hostedHydration.Lock()
	hostedHydration.status = next
	hostedHydration.Unlock()
	if hostedHydrationRevCaptured {
		capturedHostedHydrationRev.Set(capturedHostedHydrationRev.Get() + 1)
	}
}

func markHostedHydrationLoading() {
	cur := currentHostedHydration()
	if !cur.Required {
		return
	}
	setHostedHydration(hostedHydrationStatus{Required: true, Phase: hostedHydrationLoading})
}

func markHostedHydrationLocked(snapshot []byte, message string) {
	cur := currentHostedHydration()
	if !cur.Required {
		return
	}
	setHostedHydration(hostedHydrationStatus{
		Required:          true,
		Phase:             hostedHydrationLocked,
		Message:           strings.TrimSpace(message),
		EncryptedSnapshot: snapshot,
	})
}

func markHostedHydrationApplying() {
	cur := currentHostedHydration()
	if !cur.Required {
		return
	}
	setHostedHydration(hostedHydrationStatus{Required: true, Phase: hostedHydrationApplying})
}

func markHostedHydrationReady() {
	setHostedHydration(hostedHydrationStatus{Phase: hostedHydrationReady})
}

func markHostedHydrationError(message string) {
	cur := currentHostedHydration()
	if !cur.Required {
		return
	}
	setHostedHydration(hostedHydrationStatus{
		Required: true,
		Phase:    hostedHydrationError,
		Message:  strings.TrimSpace(message),
	})
}

// unlockHostedHydrationWithAppLock validates the candidate against the encrypted
// server envelope before persisting it as this device's App Lock credential.
// WebCrypto runs asynchronously and the goroutine parks while it completes, so
// PBKDF/decryption never blocks a render callback.
func unlockHostedHydrationWithAppLock(passcode string) {
	passcode = strings.TrimSpace(passcode)
	cur := currentHostedHydration()
	if cur.Phase != hostedHydrationLocked || len(cur.EncryptedSnapshot) == 0 {
		return
	}
	setHostedHydration(hostedHydrationStatus{Required: true, Phase: hostedHydrationVerifying})
	go func() {
		yieldToBrowser()
		if _, err := decryptEnvelopeSync(cur.EncryptedSnapshot, passcode); err != nil {
			markHostedHydrationLocked(cur.EncryptedSnapshot, "That App Lock passcode did not match the encrypted server data.")
			return
		}
		if !installAppLock(passcode, 0, "") {
			markHostedHydrationLocked(cur.EncryptedSnapshot, "CashFlux could not enable App Lock on this device.")
			return
		}
		markHostedHydrationLoading()
		pullActiveWorkspaceFromBackend(true)
	}()
}
