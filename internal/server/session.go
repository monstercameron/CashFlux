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
	sessionAccessTTL     = 15 * time.Minute
	sessionRefreshTTL    = 30 * 24 * time.Hour
)

type sessionClaims struct {
	Sub  string `json:"sub"`
	Type string `json:"typ"`
	Exp  int64  `json:"exp"`
}

func issueSessionToken(cfg Config, userID, tokenType string, ttl time.Duration, now time.Time) (string, error) {
	if strings.TrimSpace(userID) == "" {
		return "", fmt.Errorf("server session: user id is required")
	}
	secret := sessionSecret(cfg)
	if len(secret) == 0 {
		return "", fmt.Errorf("server session: signing secret is not configured")
	}
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, _ := json.Marshal(sessionClaims{Sub: userID, Type: tokenType, Exp: now.Add(ttl).Unix()})
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func verifySessionToken(cfg Config, token, tokenType string, now time.Time) (string, bool) {
	secret := sessionSecret(cfg)
	if len(secret) == 0 {
		return "", false
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", false
	}
	unsigned := parts[0] + "." + parts[1]
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", false
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(unsigned))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return "", false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", false
	}
	var claims sessionClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", false
	}
	if claims.Type != tokenType || strings.TrimSpace(claims.Sub) == "" || now.Unix() >= claims.Exp {
		return "", false
	}
	return claims.Sub, true
}

func sessionSecret(cfg Config) []byte {
	for _, candidate := range []string{cfg.MasterKey, cfg.Token, cfg.TokenSHA256} {
		if strings.TrimSpace(candidate) != "" {
			return []byte(candidate)
		}
	}
	return nil
}
