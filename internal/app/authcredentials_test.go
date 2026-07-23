// SPDX-License-Identifier: MIT

// Native unit tests for the password/pairing-code validation helpers
// (authcredentials.go). No build tag: these run with `go test ./internal/app/`
// on any platform without a browser or WASM runtime.
package app

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateRegisterCredentials(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		wantErr  error
	}{
		{"valid", "camr", "correcthorsebattery", nil},
		{"valid with surrounding space in username", "  camr  ", "correcthorsebattery", nil},
		{"empty username", "", "correcthorsebattery", ErrUsernameRequired},
		{"whitespace-only username", "   ", "correcthorsebattery", ErrUsernameRequired},
		{"empty password", "camr", "", ErrPasswordRequired},
		{"password too short", "camr", "short7", ErrPasswordTooShort},
		{"password exactly at floor", "camr", strings.Repeat("x", authMinPasswordLength), nil},
		{"password one under floor", "camr", strings.Repeat("x", authMinPasswordLength-1), ErrPasswordTooShort},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRegisterCredentials(tt.username, tt.password)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("validateRegisterCredentials(%q, %q) = %v, want %v", tt.username, tt.password, err, tt.wantErr)
			}
		})
	}
}

func TestValidateLoginCredentials(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		wantErr  error
	}{
		{"valid", "camr", "x", nil},
		{"short password is fine for login", "camr", "x", nil},
		{"empty username", "", "x", ErrUsernameRequired},
		{"empty password", "camr", "", ErrPasswordRequired},
		{"both empty", "", "", ErrUsernameRequired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLoginCredentials(tt.username, tt.password)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("validateLoginCredentials(%q, %q) = %v, want %v", tt.username, tt.password, err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeUsername(t *testing.T) {
	if got := normalizeUsername("  camr  "); got != "camr" {
		t.Errorf("normalizeUsername: got %q, want %q", got, "camr")
	}
	if got := normalizeUsername("camr"); got != "camr" {
		t.Errorf("normalizeUsername: got %q, want %q", got, "camr")
	}
}

func TestNormalizePairingCode(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr error
	}{
		{"plain six digits", "123456", "123456", nil},
		{"surrounding whitespace", "  123456  ", "123456", nil},
		{"grouping space in the middle", "123 456", "123456", nil},
		{"empty", "", "", ErrPairingCodeMissing},
		{"whitespace only", "   ", "", ErrPairingCodeMissing},
		{"too short", "12345", "", ErrPairingCodeInvalid},
		{"too long", "1234567", "", ErrPairingCodeInvalid},
		{"non-digit characters", "12a456", "", ErrPairingCodeInvalid},
		{"dash-separated (not stripped)", "123-456", "", ErrPairingCodeInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizePairingCode(tt.raw)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("normalizePairingCode(%q) error = %v, want %v", tt.raw, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("normalizePairingCode(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}
