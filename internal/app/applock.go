// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/applock"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// appLockKey is the localStorage key holding the passcode-lock config. It is
// user-global (not per-workspace, not redacted into a dataset) — like the OpenAI
// key — so the lock guards the whole app regardless of the active workspace.
const appLockKey = "cashflux:applock"

// loadAppLock reads the persisted lock config (a disabled zero value when absent).
func loadAppLock() applock.Config {
	var c applock.Config
	if raw := lsGet(appLockKey); raw != "" {
		_ = json.Unmarshal([]byte(raw), &c)
	}
	return c
}

func saveAppLock(c applock.Config) {
	if data, err := json.Marshal(c); err == nil {
		lsSet(appLockKey, string(data))
	}
}

// newSalt returns a fresh random hex salt (crypto/rand → crypto.getRandomValues
// under wasm). Empty only if the platform RNG fails.
func newSalt() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

// installAppLock persists an already-approved passcode as this device's App
// Lock credential. It deliberately does not rewrite or push the current dataset:
// callers such as hosted first-boot validate and hydrate the authoritative server
// snapshot before there is any local dataset worth migrating.
func installAppLock(passcode string, autoLockMinutes int, hint string) bool {
	if current := loadAppLock(); current.Enabled && activePasscode != "" && activePasscode != passcode {
		// A live passcode change must go through enableAppLock's vault migration.
		return false
	}
	salt := newSalt()
	if salt == "" {
		return false
	}
	return installAppLockWithSalt(passcode, salt, autoLockMinutes, hint)
}

// installAppLockWithSalt is goroutine-only. Its hosted-bootstrap caller must
// not report success until the WDK wrap and all legacy records are durable.
func installAppLockWithSalt(passcode, salt string, autoLockMinutes int, hint string) bool {
	config := loadAppLock().WithPasscode(passcode, salt, autoLockMinutes, hint)
	if !config.Enabled {
		return false
	}
	return runSecureLockTransition(passcode, config, false) == nil
}

// enableAppLock starts the durable lifecycle from a JS event callback. The
// setup modal is dismissed only by completeAppLockPasscodeChange's callback.
func enableAppLock(passcode string, autoLockMinutes int, hint string) bool {
	beginAppLockPasscodeChange(passcode, autoLockMinutes, hint)
	return false
}

func finishAppLockEnable() {
	legacyMigration := legacyWorkspaceNeedsProtection()
	// Remembering the passcode above lets the dataset autosave encrypt at rest
	// immediately (C45), without waiting for a reload + unlock.
	migrateDatasetAtRest() // encrypt the existing at-rest copy now
	// A pull that hit an encrypted snapshot this device could not read parked on
	// state "locked" with no way out: unlockProtectedSession only runs from the passcode
	// gate, which never renders on a device that has no lock. Setting the matching
	// passcode is the actual recovery, so re-pull here — that is the moment this
	// device becomes able to decrypt the account's data.
	if !legacyMigration && backendConnectionActive(uistate.LoadPrefs().Normalize()) {
		pullActiveWorkspaceFromBackend(true)
	}
}

var appLockPasscodeChangeInProgress bool

func beginAppLockPasscodeChange(passcode string, autoLockMinutes int, hint string) {
	if appLockPasscodeChangeInProgress || passcode == "" {
		return
	}
	salt := newSalt()
	if salt == "" {
		uistate.PostNotice(uistate.T("vault.rekeyFailed"), true)
		return
	}
	appLockPasscodeChangeInProgress = true
	suspendAutosave = true
	go func() {
		config := loadAppLock().WithPasscode(passcode, salt, autoLockMinutes, hint)
		err := runSecureLockTransition(passcode, config, false)
		if shouldOfferUnreadableVaultDiscard(err) {
			confirmDiscardUnreadableVault(passcode, salt, autoLockMinutes, hint)
			return
		}
		finishSecureAppLockChange(err, false, func() { finishAppLockEnable(); closeAppLockSetup() })
	}()
}

func closeAppLockSetup() {
	if doc := js.Global().Get("document"); doc.Truthy() {
		if modal := doc.Call("getElementById", appLockSetupID); modal.Truthy() {
			modal.Get("style").Set("display", "none")
		}
	}
	if appLockOnDone != nil {
		appLockOnDone()
	}
}

// completeAppLockPasscodeChange is also used after an explicitly confirmed
// unreadable-vault quarantine. The replacement is still one durable transition.
func completeAppLockPasscodeChange(passcode, salt string, autoLockMinutes int, hint string) {
	go func() {
		config := loadAppLock().WithPasscode(passcode, salt, autoLockMinutes, hint)
		err := runSecureLockTransition(passcode, config, false)
		finishSecureAppLockChange(err, false, func() { finishAppLockEnable(); closeAppLockSetup() })
	}()
}

func finishSecureAppLockChange(err error, disabling bool, done func()) {
	appLockPasscodeChangeInProgress = false
	if err != nil {
		if browserstore.GetString(secureLockTransitionKey) != "" {
			// The encrypted journal retains the prepared replacements across a
			// page close. Keep autosave stopped until it has been committed.
			showProtectedStorageGate(errPendingLockTransition, func() {
				suspendAutosave = false
				if done != nil {
					done()
				}
			})
		} else {
			suspendAutosave = false
		}
		key := "vault.rekeyFailed"
		if disabling {
			key = "vault.disableFailed"
		}
		uistate.PostNotice(uistate.T(key), true)
		return
	}
	suspendAutosave = false
	if done != nil {
		done()
	}
}

func confirmDiscardUnreadableVault(passcode, salt string, autoLockMinutes int, hint string) {
	raw := browserstore.GetString(credVaultStoreKey)
	if raw == "" {
		appLockPasscodeChangeInProgress = false
		suspendAutosave = false
		uistate.PostNotice(uistate.T("vault.rekeyFailed"), true)
		return
	}
	uistate.ConfirmModalLabeled(
		uistate.T("vault.changeDiscardConfirm"),
		uistate.T("vault.changeDiscardButton"), true,
		func(ok bool) {
			if !ok {
				appLockPasscodeChangeInProgress = false
				suspendAutosave = false
				return
			}
			if browserstore.GetString(credVaultStoreKey) != raw {
				appLockPasscodeChangeInProgress = false
				suspendAutosave = false
				uistate.PostNotice(uistate.T("vault.rekeyFailed"), true)
				return
			}
			key := uniqueCredVaultQuarantineKey(time.Now().Unix(), func(candidate string) bool {
				_, exists := browserstore.Get(candidate)
				return exists
			})
			browserstore.SetDurable(key, raw, func(err error) {
				if err != nil {
					appLockPasscodeChangeInProgress = false
					suspendAutosave = false
					uistate.PostNotice(uistate.T("vault.quarantineFailed"), true)
					return
				}
				browserstore.RemoveDurable(credVaultStoreKey, func(err error) {
					if err != nil {
						appLockPasscodeChangeInProgress = false
						suspendAutosave = false
						uistate.PostNotice(uistate.T("vault.quarantineFailed"), true)
						return
					}
					completeAppLockPasscodeChange(passcode, salt, autoLockMinutes, hint)
				})
			})
		},
	)
}

func uniqueCredVaultQuarantineKey(unix int64, exists func(string) bool) string {
	base := fmt.Sprintf("cashflux:quarantine:credvault:%d", unix)
	key := base
	for suffix := 1; exists != nil && exists(key); suffix++ {
		key = fmt.Sprintf("%s-%d", base, suffix)
	}
	return key
}

func shouldOfferUnreadableVaultDiscard(err error) bool {
	return errors.Is(err, errCredVaultUnreadable)
}

// disableAppLock restores the device wrap while records remain encrypted.
// Existing legacy vault entries require explicit discard consent. The inner
// v1 passcode envelope is removed — completing the reverse (decrypt) migration.
func disableAppLock(onDone ...func()) {
	loadCredVaultWithPasscode(activePasscode, func(v credVault, err error) {
		if err != nil {
			uistate.PostNotice(uistate.T("vault.disableFailed"), true)
			return
		}
		if len(v) > 0 {
			uistate.ConfirmModalLabeled(
				fmt.Sprintf(uistate.T("vault.disableConfirm"), len(v)),
				uistate.T("vault.disableConfirmButton"), true,
				func(ok bool) {
					if ok {
						removeVaultThenDisable(onDone...)
					}
				},
			)
			return
		}
		removeVaultThenDisable(onDone...)
	})
}

func removeVaultThenDisable(onDone ...func()) {
	if appLockPasscodeChangeInProgress {
		return
	}
	appLockPasscodeChangeInProgress = true
	suspendAutosave = true
	go func() {
		err := runSecureLockTransition("", applock.Config{}, true)
		finishSecureAppLockChange(err, true, func() {
			migrateDatasetAtRest()
			for _, done := range onDone {
				if done != nil {
					done()
				}
			}
		})
	}()
}

// setLockHideQuotes / setLockHideMeta flip the lock-screen content toggles on the
// current (enabled) config. No-op when the lock isn't set.
func setLockHideQuotes(hide bool) {
	if c := loadAppLock(); c.Enabled {
		c.HideQuotes = hide
		saveAppLock(c)
	}
}

func setLockHideMeta(hide bool) {
	if c := loadAppLock(); c.Enabled {
		c.HideMeta = hide
		saveAppLock(c)
	}
}

// setLockSuspended pauses or resumes the gate without dropping the passcode.
func setLockSuspended(suspended bool) {
	if c := loadAppLock(); c.Enabled {
		c.Suspended = suspended
		saveAppLock(c)
	}
}
