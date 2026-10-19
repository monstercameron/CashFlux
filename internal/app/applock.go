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
	"github.com/monstercameron/CashFlux/internal/appstate"
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

func installAppLockWithSalt(passcode, salt string, autoLockMinutes int, hint string) bool {
	// Start from the current config (not a fresh zero) so changing the passcode
	// carries over the lock-screen display prefs. On first set loadAppLock returns
	// the zero config, so the defaults still apply.
	c := loadAppLock().WithPasscode(passcode, salt, autoLockMinutes, hint)
	if !c.Enabled {
		return false
	}
	// Invalidate any existing PRF vault so a stale wrapped passcode can't be
	// used to unlock with the old credential (C282 lockout-safety invariant).
	clearPasskey()
	saveAppLock(c)
	activePasscode = passcode
	clearDatasetKeyCache()
	clearArtifactKeyCache()
	return true
}

// enableAppLock sets a passcode (with a fresh salt) and persists it. Reports
// whether it took (false for an empty passcode or RNG failure).
func enableAppLock(passcode string, autoLockMinutes int, hint string) bool {
	current := loadAppLock()
	if current.Enabled && activePasscode != "" && activePasscode != passcode {
		beginAppLockPasscodeChange(passcode, autoLockMinutes, hint)
		// The setup form remains open until the re-key has durably completed.
		return false
	}
	// Move a remembered device key before reporting that the lock is enabled.
	// The migration may need Web Crypto, so the first enable can complete from
	// its callback after this event handler has returned.
	migrationReturned := false
	completedSynchronously := false
	installed := false
	migrateAICredentialAtRest(activePasscode, passcode, func(err error) {
		completedSynchronously = !migrationReturned
		if err != nil {
			uistate.PostNotice(uistate.T("vault.rekeyFailed"), true)
			return
		}
		if !installAppLock(passcode, autoLockMinutes, hint) {
			uistate.PostNotice(uistate.T("vault.rekeyFailed"), true)
			return
		}
		installed = true
		finishAppLockEnable()
		if migrationReturned {
			if doc := js.Global().Get("document"); !doc.IsNull() && !doc.IsUndefined() {
				if modal := doc.Call("getElementById", appLockSetupID); !modal.IsNull() && !modal.IsUndefined() {
					modal.Get("style").Set("display", "none")
				}
			}
			if appLockOnDone != nil {
				appLockOnDone()
			}
		}
	})
	migrationReturned = true
	return completedSynchronously && installed
}

func finishAppLockEnable() {
	// Remembering the passcode above lets the dataset autosave encrypt at rest
	// immediately (C45), without waiting for a reload + unlock.
	migrateDatasetAtRest() // encrypt the existing at-rest copy now
	// A pull that hit an encrypted snapshot this device could not read parked on
	// state "locked" with no way out: onAppUnlocked only runs from the passcode
	// GATE, which never renders on a device that has no lock. Setting the matching
	// passcode is the actual recovery, so re-pull here — that is the moment this
	// device becomes able to decrypt the account's data.
	if uistate.LoadPrefs().Normalize().BackendActive() {
		pullActiveWorkspaceFromBackend(true)
	}
}

var appLockPasscodeChangeInProgress bool

func beginAppLockPasscodeChange(passcode string, autoLockMinutes int, hint string) {
	if appLockPasscodeChangeInProgress {
		return
	}
	salt := newSalt()
	if salt == "" {
		uistate.PostNotice(uistate.T("vault.rekeyFailed"), true)
		return
	}
	oldPasscode := activePasscode
	appLockPasscodeChangeInProgress = true
	rekeyCredVault(oldPasscode, passcode, func(err error) {
		if err != nil {
			if shouldOfferUnreadableVaultDiscard(err) {
				confirmDiscardUnreadableVault(passcode, salt, autoLockMinutes, hint)
				return
			}
			appLockPasscodeChangeInProgress = false
			uistate.PostNotice(uistate.T("vault.rekeyFailed"), true)
			return
		}
		migrateAICredentialAtRest(oldPasscode, passcode, func(err error) {
			if err != nil {
				// Restore the vault to the still-active old passcode before leaving
				// the lock untouched. A failed credential write must not strand it.
				rekeyCredVault(passcode, oldPasscode, func(rollbackErr error) {
					appLockPasscodeChangeInProgress = false
					if rollbackErr != nil {
						if app := appstate.Default; app != nil {
							app.Log().Error("restore bank-login vault after failed passcode change", "err", rollbackErr)
						}
					}
					uistate.PostNotice(uistate.T("vault.rekeyFailed"), true)
				})
				return
			}
			completeAppLockPasscodeChange(passcode, salt, autoLockMinutes, hint)
		})
	})
}

func completeAppLockPasscodeChange(passcode, salt string, autoLockMinutes int, hint string) {
	appLockPasscodeChangeInProgress = false
	if !installAppLockWithSalt(passcode, salt, autoLockMinutes, hint) {
		uistate.PostNotice(uistate.T("vault.rekeyFailed"), true)
		return
	}
	migrateDatasetAtRest()
	if uistate.LoadPrefs().Normalize().BackendActive() {
		pullActiveWorkspaceFromBackend(true)
	}
	if doc := js.Global().Get("document"); !doc.IsNull() && !doc.IsUndefined() {
		if modal := doc.Call("getElementById", appLockSetupID); !modal.IsNull() && !modal.IsUndefined() {
			modal.Get("style").Set("display", "none")
		}
	}
	if appLockOnDone != nil {
		appLockOnDone()
	}
}

func confirmDiscardUnreadableVault(passcode, salt string, autoLockMinutes int, hint string) {
	raw := browserstore.GetString(credVaultStoreKey)
	if raw == "" {
		appLockPasscodeChangeInProgress = false
		uistate.PostNotice(uistate.T("vault.rekeyFailed"), true)
		return
	}
	uistate.ConfirmModalLabeled(
		uistate.T("vault.changeDiscardConfirm"),
		uistate.T("vault.changeDiscardButton"), true,
		func(ok bool) {
			if !ok {
				appLockPasscodeChangeInProgress = false
				return
			}
			if browserstore.GetString(credVaultStoreKey) != raw {
				appLockPasscodeChangeInProgress = false
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
					uistate.PostNotice(uistate.T("vault.quarantineFailed"), true)
					return
				}
				browserstore.RemoveDurable(credVaultStoreKey, func(err error) {
					if err != nil {
						appLockPasscodeChangeInProgress = false
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

// disableAppLock removes the passcode lock and forgets the session passcode, so
// the next autosave writes plaintext — completing the reverse (decrypt) migration.
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
	oldPasscode := activePasscode
	migrateAICredentialAtRest(oldPasscode, "", func(err error) {
		if err != nil {
			uistate.PostNotice(uistate.T("vault.disableFailed"), true)
			return
		}
		removeVaultThenDisableAfterAICredential(onDone...)
	})
}

func removeVaultThenDisableAfterAICredential(onDone ...func()) {
	// Commit an empty value before dropping the lock. This makes the vault
	// unreachable only after its deletion has reached durable storage.
	browserstore.SetThen(credVaultStoreKey, "", func() {
		browserstore.Remove(credVaultStoreKey)
		clearPasskey()
		saveAppLock(applock.Config{})
		activePasscode = ""
		clearDatasetKeyCache()
		clearArtifactKeyCache()
		migrateDatasetAtRest()
		for _, done := range onDone {
			if done != nil {
				done()
			}
		}
	})
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
