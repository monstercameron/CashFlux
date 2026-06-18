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
)

// Config is the persisted app-lock configuration. The zero value is a valid,
// disabled lock (no passcode, no auto-lock).
type Config struct {
	Enabled         bool   `json:"enabled"`
	Salt            string `json:"salt"`            // random per-install, set with the passcode
	Hash            string `json:"hash"`            // hex SHA-256 of Salt+passcode
	AutoLockMinutes int    `json:"autoLockMinutes"` // 0 = lock only on reload / manual lock
}

// HashPasscode returns the hex-encoded SHA-256 of salt+passcode. Deterministic
// given the same inputs, so the salt must come from the caller.
func HashPasscode(passcode, salt string) string {
	sum := sha256.Sum256([]byte(salt + passcode))
	return hex.EncodeToString(sum[:])
}

// WithPasscode returns a copy of the config with the lock enabled for the given
// passcode (hashed with salt) and auto-lock window. An empty passcode or salt is
// rejected (returns the config unchanged) so the lock can't be enabled without a
// real secret. A negative auto-lock window is clamped to 0 (manual/reload only).
func (c Config) WithPasscode(passcode, salt string, autoLockMinutes int) Config {
	if passcode == "" || salt == "" {
		return c
	}
	if autoLockMinutes < 0 {
		autoLockMinutes = 0
	}
	return Config{
		Enabled:         true,
		Salt:            salt,
		Hash:            HashPasscode(passcode, salt),
		AutoLockMinutes: autoLockMinutes,
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
	return c.Enabled && c.AutoLockMinutes > 0 && idleMinutes >= c.AutoLockMinutes
}
