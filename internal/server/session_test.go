// SPDX-License-Identifier: MIT

package server

import (
	"testing"
	"time"
)

// TestSessionKeySeparation: with a dedicated SessionKey set, tokens are signed
// and verified under it — and a token forged under the AES MasterKey does NOT
// verify. This is the whole point of the separation: compromising/rotating one
// secret must not affect the other.
func TestSessionKeySeparation(t *testing.T) {
	now := time.Now().UTC()
	cfg := Config{SessionKey: "session-signing-secret-abcdef0123456789", MasterKey: "0123456789abcdef0123456789abcdef"}

	tok, err := issueSessionToken(cfg, "github:1", "access", time.Hour, now)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if sub, ok := verifySessionToken(cfg, tok, "access", now); !ok || sub != "github:1" {
		t.Fatalf("verify under SessionKey = %q ok=%v, want github:1 true", sub, ok)
	}

	// A token minted when only MasterKey was the secret must NOT verify once a
	// dedicated SessionKey is in force (different HMAC key).
	masterOnly := Config{MasterKey: "0123456789abcdef0123456789abcdef"}
	legacyTok, err := issueSessionToken(masterOnly, "github:1", "access", time.Hour, now)
	if err != nil {
		t.Fatalf("issue legacy: %v", err)
	}
	if _, ok := verifySessionToken(cfg, legacyTok, "access", now); ok {
		t.Fatal("token signed under MasterKey verified under a separate SessionKey — keys are not isolated")
	}
}

// TestSessionKeyRotation: after rotating SessionKey (old → SessionKeyPrevious,
// new → SessionKey), tokens signed under the OLD key still verify for the window,
// and new tokens sign under the NEW key.
func TestSessionKeyRotation(t *testing.T) {
	now := time.Now().UTC()
	oldKey := "old-session-key-000000000000000000000000"
	newKey := "new-session-key-111111111111111111111111"

	before := Config{SessionKey: oldKey}
	oldTok, err := issueSessionToken(before, "github:7", "refresh", time.Hour, now)
	if err != nil {
		t.Fatalf("issue old: %v", err)
	}

	// Rotated config: new key signs; old key accepted on verify only.
	after := Config{SessionKey: newKey, SessionKeyPrevious: oldKey}
	if sub, ok := verifySessionToken(after, oldTok, "refresh", now); !ok || sub != "github:7" {
		t.Fatalf("old token verify post-rotation = %q ok=%v, want github:7 true", sub, ok)
	}
	newTok, err := issueSessionToken(after, "github:7", "refresh", time.Hour, now)
	if err != nil {
		t.Fatalf("issue new: %v", err)
	}
	// The new token must NOT verify under the pre-rotation config (only old key).
	if _, ok := verifySessionToken(before, newTok, "refresh", now); ok {
		t.Fatal("token signed under the new key verified under the old-only config")
	}
	// Once the previous key is dropped, the old token stops verifying.
	dropped := Config{SessionKey: newKey}
	if _, ok := verifySessionToken(dropped, oldTok, "refresh", now); ok {
		t.Fatal("old token still verified after the previous key was removed")
	}
}

// TestSessionSigningFallback: with no SessionKey, signing falls back to MasterKey
// (non-breaking for existing deployments).
func TestSessionSigningFallback(t *testing.T) {
	now := time.Now().UTC()
	cfg := Config{MasterKey: "0123456789abcdef0123456789abcdef"}
	tok, err := issueSessionToken(cfg, "github:2", "access", time.Hour, now)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, ok := verifySessionToken(cfg, tok, "access", now); !ok {
		t.Fatal("MasterKey fallback signing/verify failed")
	}
	if len(sessionSigningSecret(Config{})) != 0 {
		t.Fatal("no-secret config produced a signing secret")
	}
}
