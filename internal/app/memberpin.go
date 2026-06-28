// SPDX-License-Identifier: MIT

//go:build js && wasm

// memberpin.go — package-level PIN helpers for C274 (local per-member profile
// + PIN switch). These are thin wrappers around the App methods defined in
// internal/appstate/memberpin_js.go, so code inside the app package can call
// them without threading appstate.Default through every call site.
package app

import "github.com/monstercameron/CashFlux/internal/appstate"

// SetMemberPIN validates the PIN strength (StrengthTooShort and StrengthWeak
// are rejected) and stores a PBKDF2-SHA256 hash for the given member.
// Returns an error when the PIN is too weak or the RNG fails.
func SetMemberPIN(memberID, pin string) error {
	if appstate.Default == nil {
		return nil
	}
	return appstate.Default.SetMemberPIN(memberID, pin)
}

// ClearMemberPIN removes the stored PIN for the given member (no-op if none).
func ClearMemberPIN(memberID string) {
	if appstate.Default != nil {
		appstate.Default.ClearMemberPIN(memberID)
	}
}

// MemberHasPIN reports whether the given member currently has a PIN set.
func MemberHasPIN(memberID string) bool {
	if appstate.Default == nil {
		return false
	}
	return appstate.Default.MemberHasPIN(memberID)
}

// VerifyMemberPIN checks the supplied PIN against the stored PBKDF2 hash.
// Returns false for unknown members, empty PINs, or incorrect PINs.
func VerifyMemberPIN(memberID, pin string) bool {
	if appstate.Default == nil {
		return false
	}
	return appstate.Default.VerifyMemberPIN(memberID, pin)
}
