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
	secret := sessionSecret(cfg)
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
	secret := sessionSecret(cfg)
	if len(secret) == 0 {
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
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(unsigned))
	if !hmac.Equal(sig, mac.Sum(nil)) {
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

func sessionSecret(cfg Config) []byte {
	for _, candidate := range []string{cfg.MasterKey, cfg.Token, cfg.TokenSHA256} {
		if strings.TrimSpace(candidate) != "" {
			return []byte(candidate)
		}
	}
	return nil
}
