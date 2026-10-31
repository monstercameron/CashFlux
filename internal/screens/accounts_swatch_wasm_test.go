// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestInstitutionSwatchColorOnlyPassesPlainHex(t *testing.T) {
	cases := []struct {
		name, color, want string
	}{
		{"six digit hex", "#1a2B3c", "#1a2B3c"},
		{"three digit hex", "#abc", "#abc"},
		{"blank falls back", "  ", "#7c83ff"},
		{"no hash", "1a2b3c", "#7c83ff"},
		{"css injection", "red;}body{display:none", "#7c83ff"},
		{"wrong length", "#12", "#7c83ff"},
		{"non hex digit", "#ggg", "#7c83ff"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := institutionSwatchColor(domain.Institution{Color: tc.color})
			if got != tc.want {
				t.Fatalf("institutionSwatchColor(%q) = %q, want %q", tc.color, got, tc.want)
			}
		})
	}
}
