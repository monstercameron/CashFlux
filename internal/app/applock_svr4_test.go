// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"errors"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/browserstore"
)

func TestSVR4UnreadableVaultDiscardDecision(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "unreadable vault", err: errCredVaultUnreadable, want: true},
		{name: "wrapped unreadable vault", err: errors.Join(errors.New("rekey failed"), errCredVaultUnreadable), want: true},
		{name: "encryption failure", err: errors.New("encryption failed"), want: false},
		{name: "nil", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldOfferUnreadableVaultDiscard(tt.err); got != tt.want {
				t.Fatalf("shouldOfferUnreadableVaultDiscard(%v) = %t, want %t", tt.err, got, tt.want)
			}
		})
	}
}

func TestSVR4CredentialVaultQuarantineKeyNeverOverwrites(t *testing.T) {
	occupied := map[string]bool{
		"cashflux:quarantine:credvault:123":   true,
		"cashflux:quarantine:credvault:123-1": true,
	}
	got := uniqueCredVaultQuarantineKey(123, func(key string) bool { return occupied[key] })
	if want := "cashflux:quarantine:credvault:123-2"; got != want {
		t.Fatalf("uniqueCredVaultQuarantineKey() = %q, want %q", got, want)
	}
}

func TestSVR4MalformedVaultRekeyOffersExplicitDiscardPath(t *testing.T) {
	const corrupt = "not-base64-vault-bytes"
	browserstore.Set(credVaultStoreKey, corrupt)
	t.Cleanup(func() { browserstore.Remove(credVaultStoreKey) })

	result := make(chan error, 1)
	rekeyCredVault("old-passcode", "new-passcode", func(err error) { result <- err })
	select {
	case err := <-result:
		if !shouldOfferUnreadableVaultDiscard(err) {
			t.Fatalf("rekey error %v should offer explicit discard recovery", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for malformed vault classification")
	}
	if got := browserstore.GetString(credVaultStoreKey); got != corrupt {
		t.Fatalf("malformed vault changed before confirmation: got %q, want %q", got, corrupt)
	}
}
