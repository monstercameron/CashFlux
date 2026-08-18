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

func TestAuthWatchFiresOncePerCredentialNotOncePerWrite(t *testing.T) {
	// A rotation writes BOTH the access and the refresh key. Reacting to writes
	// made every other tab drop its socket, rebuild the watch stream and
	// re-flush the queue twice for one rotation. Reacting to the CREDENTIAL
	// collapses that back to one, whichever order the two writes land in.
	browserstore.Remove(AuthAccessTokenKey)
	browserstore.Remove(AuthRefreshTokenKey)
	t.Cleanup(func() {
		browserstore.Remove(AuthAccessTokenKey)
		browserstore.Remove(AuthRefreshTokenKey)
		authWatchStarted = false
	})

	authWatchStarted = false
	fired := 0
	WatchAuthAcrossTabs(func() { fired++ })

	// One rotation, delivered as two key notifications.
	browserstore.Set(AuthAccessTokenKey, "rotated-1")
	browserstore.Set(AuthRefreshTokenKey, "refresh-1")
	notifyAuthWatchersForTest(AuthAccessTokenKey)
	notifyAuthWatchersForTest(AuthRefreshTokenKey)
	if fired != 1 {
		t.Fatalf("fired %d times for one rotation, want 1", fired)
	}

	// A second, genuinely different credential fires again.
	browserstore.Set(AuthAccessTokenKey, "rotated-2")
	notifyAuthWatchersForTest(AuthAccessTokenKey)
	if fired != 2 {
		t.Fatalf("fired %d times, want 2 after a second rotation", fired)
	}

	// A repeat notification for an unchanged credential does nothing.
	notifyAuthWatchersForTest(AuthRefreshTokenKey)
	if fired != 2 {
		t.Fatalf("fired %d times, want 2 — an unchanged credential must not churn", fired)
	}
}
