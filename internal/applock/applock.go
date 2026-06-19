// Package applock models CashFlux's optional passcode lock: a soft gate that
// keeps the app's screens behind a passcode and can auto-lock after a period of
// inactivity. It is a deterrent, not encryption — the data still lives in the
// browser's local storage — so the passcode is never stored in the clear; only a
// salted SHA-256 hash is kept.
//
// Pure Go, no platform dependencies (the random salt and the wall clock are the
// caller's job, so this stays deterministic and unit-testable). The wasm/UI layer
// generates the salt (crypto/rand), measures idle time, and renders the gate.
package applock

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
)

// Config is the persisted app-lock configuration. The zero value is a valid,
// disabled lock (no passcode, no auto-lock).
type Config struct {
	Enabled         bool   `json:"enabled"`
	Salt            string `json:"salt"`            // random per-install, set with the passcode
	Hash            string `json:"hash"`            // hex SHA-256 of Salt+passcode
	AutoLockMinutes int    `json:"autoLockMinutes"` // 0 = lock only on reload / manual lock
	Hint            string `json:"hint,omitempty"`  // optional reminder, revealed only after failed tries
	// Lock-screen content toggles. Stored as "hide" flags so the default (zero
	// value / older configs) is "shown" — both default ON per the B17.1 spec.
	HideQuotes bool `json:"hideQuotes,omitempty"`
	HideMeta   bool `json:"hideMeta,omitempty"`
	// Suspended pauses the gate without dropping the passcode: the credentials are
	// kept, but the lock screen doesn't appear. Resuming needs no new passcode.
	Suspended bool `json:"suspended,omitempty"`
}

// Active reports whether the gate should actually guard the app: a passcode is set
// and the lock isn't paused.
func (c Config) Active() bool { return c.Enabled && !c.Suspended }

// ValidHint reports whether hint is safe to store with the given passcode. An
// empty hint (no hint) is always fine; a non-empty hint must not contain the
// passcode (case-insensitive), so it can never leak the secret.
func ValidHint(hint, passcode string) bool {
	hint = strings.TrimSpace(hint)
	if hint == "" {
		return true
	}
	if passcode == "" {
		return false
	}
	return !strings.Contains(strings.ToLower(hint), strings.ToLower(passcode))
}

// HashPasscode returns the hex-encoded SHA-256 of salt+passcode. Deterministic
// given the same inputs, so the salt must come from the caller.
func HashPasscode(passcode, salt string) string {
	sum := sha256.Sum256([]byte(salt + passcode))
	return hex.EncodeToString(sum[:])
}

// WithPasscode returns a copy of the config with the lock enabled for the given
// passcode (hashed with salt), auto-lock window, and optional hint. An empty
// passcode or salt is rejected (returns the config unchanged) so the lock can't
// be enabled without a real secret. A negative auto-lock window is clamped to 0
// (manual/reload only). A hint that would leak the passcode is dropped. The
// lock-screen display preferences (HideQuotes/HideMeta) are carried over from the
// receiver — they're unrelated to the credential, so changing the passcode must
// not silently reset them. The lock is left active (un-suspended), since setting
// a passcode is an explicit re-enable.
func (c Config) WithPasscode(passcode, salt string, autoLockMinutes int, hint string) Config {
	if passcode == "" || salt == "" {
		return c
	}
	if autoLockMinutes < 0 {
		autoLockMinutes = 0
	}
	if !ValidHint(hint, passcode) {
		hint = ""
	}
	return Config{
		Enabled:         true,
		Salt:            salt,
		Hash:            HashPasscode(passcode, salt),
		AutoLockMinutes: autoLockMinutes,
		Hint:            strings.TrimSpace(hint),
		HideQuotes:      c.HideQuotes,
		HideMeta:        c.HideMeta,
	}
}

// Cleared returns a disabled lock (no passcode), for turning the lock off.
func (c Config) Cleared() Config { return Config{} }

// Verify reports whether passcode matches the configured hash. Always false when
// the lock is disabled or unconfigured. Uses a constant-time comparison so a
// wrong guess can't be timed against the real hash.
func (c Config) Verify(passcode string) bool {
	if !c.Enabled || c.Hash == "" || c.Salt == "" {
		return false
	}
	got := HashPasscode(passcode, c.Salt)
	return subtle.ConstantTimeCompare([]byte(got), []byte(c.Hash)) == 1
}

// ShouldAutoLock reports whether the app should auto-lock given how many whole
// minutes the user has been idle. Only fires when the lock is enabled with a
// positive auto-lock window.
func (c Config) ShouldAutoLock(idleMinutes int) bool {
	return c.Active() && c.AutoLockMinutes > 0 && idleMinutes >= c.AutoLockMinutes
}
