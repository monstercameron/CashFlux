// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/GoWebComponents/v5/state"
)

// One answer to "is this device signed in?".
//
// There were two, and they disagreed. Sync asked EffectiveServerToken, which
// prefers the rotated access token a Custom Sync session stores in
// browserstore. The Cloud settings pane asked prefs.ServerToken directly. Those
// are different credentials with different lifetimes, so the pane could tell a
// device with a working, actively-syncing session that it needed to request
// access — while simultaneously offering "Set a password", which is only ever
// shown to a device that is ALREADY authorized. Two contradictory prompts on one
// screen is the visible form of two definitions of the same fact.
//
// The rule: a device is signed in if it holds a credential the sync layer would
// actually send. Anything deciding whether to show a sign-in surface asks here.

// CredentialSource says which credential a session is running on. It is reported
// rather than hidden because the two behave differently — a rotating credential
// can be refreshed and revoked server-side, a static one cannot — and a support
// conversation goes differently depending on which one a device holds.
type CredentialSource string

const (
	// CredentialNone means this device holds nothing it could authenticate with.
	CredentialNone CredentialSource = ""
	// CredentialRotating is a Custom Sync session's access token. It lives in
	// browserstore, rotates, and is the credential sync actually sends.
	CredentialRotating CredentialSource = "rotating"
	// CredentialStatic is the self-host bearer token from preferences. It never
	// rotates and is only used when no rotating session exists.
	CredentialStatic CredentialSource = "static"
)

// SessionState is what this device is holding right now.
type SessionState struct {
	Source CredentialSource
	// Token is the bearer the sync layer would send. Never rendered.
	Token string
}

// Present reports whether a credential exists at all.
func (s SessionState) Present() bool { return s.Source != CredentialNone }

// Session resolves the credential this device would authenticate with, given the
// static token from preferences.
func Session(staticToken string) SessionState {
	if t := strings.TrimSpace(browserstore.GetString(AuthAccessTokenKey)); t != "" {
		return SessionState{Source: CredentialRotating, Token: t}
	}
	if t := strings.TrimSpace(staticToken); t != "" {
		return SessionState{Source: CredentialStatic, Token: t}
	}
	return SessionState{Source: CredentialNone}
}

// authRevAtomID ticks whenever the stored credential changes — including from
// ANOTHER TAB. Surfaces that show sign-in state subscribe to it so they re-render
// on a sign-in, a sign-out or a rotation that happened somewhere else, instead of
// showing whatever was true when that tab happened to boot.
const authRevAtomID = "auth:revision"

// UseAuthRevision subscribes the calling component to credential changes.
func UseAuthRevision() state.Atom[int] { return state.UseAtom(authRevAtomID, 0) }

var (
	capturedAuthRev  state.Atom[int]
	authRevCaptured  bool
	authWatchStarted bool
)

// CaptureAuthRevision records the atom so cross-tab callbacks — which run outside
// any render — can bump it.
func CaptureAuthRevision(a state.Atom[int]) {
	capturedAuthRev = a
	authRevCaptured = true
}

// BumpAuthRevision re-renders every surface subscribed to the credential.
func BumpAuthRevision() {
	if authRevCaptured {
		capturedAuthRev.Set(capturedAuthRev.Get() + 1)
	}
}

// WatchAuthAcrossTabs makes credential changes in other tabs visible in this one.
//
// onChange runs after this tab's cached copy has been updated, so a caller can
// react to the NEW credential — dropping a socket authenticated as the old one,
// clearing a stale "your credentials were rejected" flag, re-rendering a pane
// that is claiming the user is signed out. Idempotent.
func WatchAuthAcrossTabs(onChange func()) {
	if authWatchStarted {
		return
	}
	authWatchStarted = true
	// A rotation writes BOTH keys, so a naive watcher fires twice for one event
	// and every other tab drops its socket, rebuilds the watch stream and
	// re-flushes the queue twice over. Bounded, but pure churn — and it doubles
	// under a burst of refreshes. Adversarial review, 2026-08-18.
	//
	// The fix is to react to the CREDENTIAL rather than to the writes: whatever
	// order the two keys land in, this fires once per distinct bearer.
	lastSeen := strings.TrimSpace(browserstore.GetString(AuthAccessTokenKey))
	for _, key := range []string{AuthAccessTokenKey, AuthRefreshTokenKey} {
		browserstore.Watch(key, func(string, bool) {
			current := strings.TrimSpace(browserstore.GetString(AuthAccessTokenKey))
			if current == lastSeen {
				return
			}
			lastSeen = current
			BumpAuthRevision()
			if onChange != nil {
				onChange()
			}
		})
	}
}

// notifyAuthWatchersForTest delivers a credential-change notification the way a
// BroadcastChannel message would, so the coalescing rule can be tested without a
// second browser tab.
func notifyAuthWatchersForTest(key string) { browserstore.NotifyForTest(key) }
