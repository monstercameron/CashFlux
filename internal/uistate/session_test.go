// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/browserstore"
)

// The bug this encodes: the Cloud pane decided "signed in" from the STATIC
// preference token while sync authenticated with the ROTATING one. A device
// holding only a rotating session — which is every device paired by approval or
// by an activation code — was told to request access while it was syncing
// happily, and offered "Set a password" in the same breath, which is only shown
// to a device that is already authorized.

func TestSessionPrefersTheCredentialSyncActuallySends(t *testing.T) {
	browserstore.Remove(AuthAccessTokenKey)
	t.Cleanup(func() { browserstore.Remove(AuthAccessTokenKey) })

	// A rotating session with NO static token is the hosted/paired case that was
	// being reported as signed out.
	browserstore.Set(AuthAccessTokenKey, "rotating-abc")
	s := Session("")
	if !s.Present() {
		t.Fatal("a device holding a rotating session must read as signed in")
	}
	if s.Source != CredentialRotating || s.Token != "rotating-abc" {
		t.Fatalf("session = %+v, want the rotating credential", s)
	}

	// The rotating credential outranks a stale static one, matching what
	// EffectiveServerToken sends on the wire — the two must not disagree.
	s = Session("static-xyz")
	if s.Source != CredentialRotating || s.Token != "rotating-abc" {
		t.Fatalf("session = %+v, want the rotating credential to win", s)
	}
	if s.Token != EffectiveServerToken("static-xyz") {
		t.Fatalf("Session and EffectiveServerToken disagree: %q vs %q", s.Token, EffectiveServerToken("static-xyz"))
	}
}

func TestSessionFallsBackToTheStaticToken(t *testing.T) {
	browserstore.Remove(AuthAccessTokenKey)
	t.Cleanup(func() { browserstore.Remove(AuthAccessTokenKey) })

	s := Session("static-xyz")
	if !s.Present() || s.Source != CredentialStatic || s.Token != "static-xyz" {
		t.Fatalf("session = %+v, want the self-host token", s)
	}
	if s.Token != EffectiveServerToken("static-xyz") {
		t.Fatalf("Session and EffectiveServerToken disagree: %q vs %q", s.Token, EffectiveServerToken("static-xyz"))
	}
}

func TestSessionIsAbsentOnlyWhenThereIsNoCredentialAtAll(t *testing.T) {
	browserstore.Remove(AuthAccessTokenKey)
	t.Cleanup(func() { browserstore.Remove(AuthAccessTokenKey) })

	if s := Session(""); s.Present() || s.Source != CredentialNone {
		t.Fatalf("session = %+v, want none", s)
	}
	// Whitespace is not a credential; treating it as one would show a signed-out
	// device the signed-in surface, which is the same class of error inverted.
	if s := Session("   "); s.Present() {
		t.Fatalf("session = %+v, want none for a blank token", s)
	}
	browserstore.Set(AuthAccessTokenKey, "   ")
	if s := Session(""); s.Present() {
		t.Fatalf("session = %+v, want none for a blank rotating token", s)
	}
}
