// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"

	"github.com/monstercameron/CashFlux/internal/applock"
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

// enableAppLock sets a passcode (with a fresh salt) and persists it. Reports
// whether it took (false for an empty passcode or RNG failure).
func enableAppLock(passcode string, autoLockMinutes int, hint string) bool {
	salt := newSalt()
	if salt == "" {
		return false
	}
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
	// Remember the passcode for the session so the dataset autosave can encrypt at
	// rest immediately (C45), without waiting for a reload + unlock.
	activePasscode = passcode
	migrateDatasetAtRest() // encrypt the existing at-rest copy now
	return true
}

// disableAppLock removes the passcode lock and forgets the session passcode, so
// the next autosave writes plaintext — completing the reverse (decrypt) migration.
func disableAppLock() {
	// Remove the PRF vault first (C282): once the lock is gone there is no
	// passcode to wrap, so the vault is meaningless. Clear before wiping the
	// config so an interrupted reload can't leave a dangling vault.
	clearPasskey()
	saveAppLock(applock.Config{})
	activePasscode = ""
	migrateDatasetAtRest() // rewrite the at-rest copy as plaintext now
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
