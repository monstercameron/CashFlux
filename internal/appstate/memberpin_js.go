// SPDX-License-Identifier: MIT

//go:build js && wasm

// memberpin_js.go — per-member PIN storage as App methods (C274).
//
// Kept in a separate build-tagged file so the browserstore / crypto/rand
// imports stay out of the pure-Go native test build.  All PIN data lives in
// browserstore (not the SQLite dataset) so it survives a financial-data wipe
// when the "cashflux:member-pins" key is present in app.keptOnWipeKeys.
package appstate

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/monstercameron/CashFlux/internal/applock"
	"github.com/monstercameron/CashFlux/internal/browserstore"
)

// MemberPinsKey is the browserstore key for the per-member PIN map.
// Exported so the app package can add it to keptOnWipeKeys.
const MemberPinsKey = "cashflux:member-pins"

// memberPINRecord holds the PBKDF2 hash and random salt for one member.
type memberPINRecord struct {
	Hash string `json:"hash"`
	Salt string `json:"salt"`
}

func loadMemberPins() map[string]memberPINRecord {
	raw := browserstore.GetString(MemberPinsKey)
	if raw == "" {
		return map[string]memberPINRecord{}
	}
	var m map[string]memberPINRecord
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return map[string]memberPINRecord{}
	}
	return m
}

func saveMemberPins(m map[string]memberPINRecord) {
	if data, err := json.Marshal(m); err == nil {
		browserstore.Set(MemberPinsKey, string(data))
	}
}

// newMemberPINSalt returns a fresh 16-byte hex salt from crypto/rand.
// Returns "" on RNG failure (extremely unlikely on any supported platform).
func newMemberPINSalt() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

// SetMemberPIN validates PIN strength (rejects StrengthTooShort and
// StrengthWeak per the applock policy) and stores a PBKDF2-SHA256 hash.
func (a *App) SetMemberPIN(memberID, pin string) error {
	switch applock.PasscodeStrength(pin) {
	case applock.StrengthTooShort, applock.StrengthWeak:
		return errors.New("pin too weak — must be at least 4 characters and not too simple")
	}
	salt := newMemberPINSalt()
	if salt == "" {
		return errors.New("rng failure — cannot generate PIN salt")
	}
	hash := applock.HashPasscodePBKDF2(pin, salt)
	m := loadMemberPins()
	m[memberID] = memberPINRecord{Hash: hash, Salt: salt}
	saveMemberPins(m)
	return nil
}

// ClearMemberPIN removes the stored PIN for the given member (no-op if none).
func (a *App) ClearMemberPIN(memberID string) {
	m := loadMemberPins()
	delete(m, memberID)
	saveMemberPins(m)
}

// MemberHasPIN reports whether the given member currently has a PIN set.
func (a *App) MemberHasPIN(memberID string) bool {
	m := loadMemberPins()
	_, ok := m[memberID]
	return ok
}

// VerifyMemberPIN checks the supplied PIN against the stored PBKDF2 hash.
// Returns false for unknown members, empty PINs, or wrong PINs.
func (a *App) VerifyMemberPIN(memberID, pin string) bool {
	if pin == "" {
		return false
	}
	m := loadMemberPins()
	rec, ok := m[memberID]
	if !ok {
		return false
	}
	match, _, _ := applock.VerifyPasscode(pin, rec.Salt, rec.Hash)
	return match
}
