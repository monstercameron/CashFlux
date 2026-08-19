// SPDX-License-Identifier: MIT

package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	sessionRefreshCookie = "cashflux_refresh"
	sessionCSRFCookie    = "cashflux_csrf"
	sessionCSRFHeader    = "X-CashFlux-CSRF"
	sessionAccessTTL     = 15 * time.Minute
	sessionRefreshTTL    = 30 * 24 * time.Hour
)

type sessionClaims struct {
	Sub    string `json:"sub"`
	Type   string `json:"typ"`
	Exp    int64  `json:"exp"`
	JTI    string `json:"jti,omitempty"`
	Family string `json:"fam,omitempty"`
}

func issueSessionToken(cfg Config, userID, tokenType string, ttl time.Duration, now time.Time) (string, error) {
	return issueSessionTokenWithClaims(cfg, sessionClaims{Sub: userID, Type: tokenType, Exp: now.Add(ttl).Unix()})
}

func issueSessionTokenWithClaims(cfg Config, claims sessionClaims) (string, error) {
	if strings.TrimSpace(claims.Sub) == "" {
		return "", fmt.Errorf("server session: user id is required")
	}
	secret := sessionSigningSecret(cfg)
	if len(secret) == 0 {
		return "", fmt.Errorf("server session: signing secret is not configured")
	}
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, _ := json.Marshal(claims)
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func verifySessionToken(cfg Config, token, tokenType string, now time.Time) (string, bool) {
	claims, ok := verifySessionClaims(cfg, token, tokenType, now)
	if !ok {
		return "", false
	}
	return claims.Sub, true
}

func verifySessionClaims(cfg Config, token, tokenType string, now time.Time) (sessionClaims, bool) {
	secrets := sessionVerifySecrets(cfg)
	if len(secrets) == 0 {
		return sessionClaims{}, false
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return sessionClaims{}, false
	}
	unsigned := parts[0] + "." + parts[1]
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return sessionClaims{}, false
	}
	// Accept a signature under ANY configured verify secret (current + previous),
	// so rotating the session key doesn't invalidate live tokens for one window.
	// Every candidate is tried with hmac.Equal (constant time); no early return on
	// the first mismatch leaks timing about which key matched.
	matched := false
	for _, secret := range secrets {
		mac := hmac.New(sha256.New, secret)
		_, _ = mac.Write([]byte(unsigned))
		if hmac.Equal(sig, mac.Sum(nil)) {
			matched = true
		}
	}
	if !matched {
		return sessionClaims{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return sessionClaims{}, false
	}
	var claims sessionClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return sessionClaims{}, false
	}
	if claims.Type != tokenType || strings.TrimSpace(claims.Sub) == "" || now.Unix() >= claims.Exp {
		return sessionClaims{}, false
	}
	return claims, true
}

func sessionTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// sessionSigningSecret returns the single secret used to SIGN new session tokens.
//
// The candidates are deliberately only server-side secrets. This chain used to
// end at cfg.Token and cfg.TokenSHA256 — the static bearer token every syncing
// client is given, and its hash — which made the signing key a value the
// clients themselves hold. Anyone with the sync token could therefore mint a
// session JWT for any `sub` on the server, the owner account included, and read
// every account's data as that user. There is no derivation that fixes it: any
// key computed from the token is equally computable by every token holder, so
// the token simply cannot participate.
//
// When neither key is configured, ResolveSessionKey fills SessionKey in from a
// secret the server generated and kept to itself (Store.EnsureServerSessionSecret),
// which is why an empty return here means "no session can be issued" rather than
// "fall back to something". Failing closed is the point: a deployment that
// somehow reaches signing with no server-side secret must refuse to mint
// credentials, not invent a guessable one.
func sessionSigningSecret(cfg Config) []byte {
	for _, candidate := range []string{cfg.SessionKey, cfg.MasterKey} {
		if strings.TrimSpace(candidate) != "" {
			return []byte(candidate)
		}
	}
	return nil
}

// sessionVerifySecrets returns every secret accepted when VERIFYING a token: the
// current signing secret plus the optional previous session key (rotation
// window). Order does not matter — verifySessionClaims tries each with a
// constant-time compare.
func sessionVerifySecrets(cfg Config) [][]byte {
	var secrets [][]byte
	if primary := sessionSigningSecret(cfg); len(primary) > 0 {
		secrets = append(secrets, primary)
	}
	if prev := strings.TrimSpace(cfg.SessionKeyPrevious); prev != "" {
		secrets = append(secrets, []byte(prev))
	}
	return secrets
}

// ResolveSessionKey fills in a server-generated session-signing secret when the
// operator has configured neither CASHFLUX_SERVER_SESSION_KEY nor
// CASHFLUX_SERVER_MASTER_KEY, and returns the config to use.
//
// Every constructor that pairs a Config with a Store calls this, because those
// are exactly the deployments that can issue sessions. It is idempotent and
// cheap: the secret is generated once and persisted, so restarts keep signing
// with the same key and outstanding sessions survive a deploy.
//
// A configured key always wins and is never overwritten — this only fills a
// vacuum that previously got filled by the client-held bearer token.
func ResolveSessionKey(cfg Config, store *Store) (Config, error) {
	if strings.TrimSpace(cfg.SessionKey) != "" || strings.TrimSpace(cfg.MasterKey) != "" {
		return cfg, nil
	}
	if store == nil {
		return cfg, nil
	}
	secret, err := store.EnsureServerSessionSecret(time.Now().UTC())
	if err != nil {
		return cfg, fmt.Errorf("server session: resolve signing key: %w", err)
	}
	cfg.SessionKey = secret
	return cfg, nil
}
