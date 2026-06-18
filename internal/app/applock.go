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
	c := applock.Config{}.WithPasscode(passcode, salt, autoLockMinutes, hint)
	if !c.Enabled {
		return false
	}
	saveAppLock(c)
	return true
}

// disableAppLock removes the passcode lock.
func disableAppLock() { saveAppLock(applock.Config{}) }

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
