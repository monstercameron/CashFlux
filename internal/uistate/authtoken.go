// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/browserstore"
)

// AuthAccessTokenKey holds this device's ROTATED access token for a Custom Sync
// session. It is the live credential: a hosted session rotates its bearer, and
// the rotated value lands here rather than in prefs, which keeps the static
// self-host token.
const AuthAccessTokenKey = "cashflux:auth:access-token"

// EffectiveServerToken returns the bearer token a backend RPC must send.
//
// The rotated access token wins when a Custom Sync session exists; otherwise the
// caller's static token from prefs (self-host token mode, which never rotates).
//
// It exists in uistate rather than in internal/app because both sides need it and
// only one of them could have it: internal/app imports internal/screens, so the
// screens cannot reach back for internal/app's copy. That asymmetry is exactly how
// every Smart+ feature came to send prefs.ServerToken directly — the rule was
// written down in the one package the callers could not import, so the callers
// each did the plausible wrong thing instead.
//
// On a hosted instance prefs.ServerToken is stale or empty, so sending it gets
// `codes.Unauthenticated, "invalid bearer token"` from the server (internal/server
// auth.go) — visible to the user as "invalid bearer token" under a category
// suggestion, while features that call OpenAI directly keep working and make it
// look like the AI key is at fault.
func EffectiveServerToken(staticToken string) string {
	if t := strings.TrimSpace(browserstore.GetString(AuthAccessTokenKey)); t != "" {
		return t
	}
	return staticToken
}
