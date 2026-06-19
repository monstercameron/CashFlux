package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const oauthStateCookie = "cashflux_oauth_state"

func handleOAuthStart(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		providerName := strings.TrimSpace(r.PathValue("provider"))
		provider, ok := cfg.OAuthProviders[providerName]
		if !ok {
			http.Error(w, "oauth provider is not configured", http.StatusNotFound)
			return
		}
		authURL := oauthAuthURL(providerName)
		if authURL == "" {
			http.Error(w, "oauth provider is not supported", http.StatusNotFound)
			return
		}
		state, err := randomURLToken(32)
		if err != nil {
			http.Error(w, "build oauth state", http.StatusInternalServerError)
			return
		}
		verifier, err := randomURLToken(48)
		if err != nil {
			http.Error(w, "build oauth verifier", http.StatusInternalServerError)
			return
		}
		challenge := pkceChallenge(verifier)
		http.SetCookie(w, &http.Cookie{
			Name:     oauthStateCookie,
			Value:    state + "." + verifier,
			Path:     "/v1/auth/" + providerName + "/callback",
			HttpOnly: true,
			Secure:   requestIsSecure(r),
			SameSite: http.SameSiteLaxMode,
			Expires:  time.Now().Add(10 * time.Minute),
		})
		u, _ := url.Parse(authURL)
		q := u.Query()
		q.Set("client_id", provider.ClientID)
		q.Set("redirect_uri", provider.RedirectURL)
		q.Set("response_type", "code")
		q.Set("state", state)
		q.Set("code_challenge", challenge)
		q.Set("code_challenge_method", "S256")
		q.Set("scope", oauthScope(providerName))
		u.RawQuery = q.Encode()
		http.Redirect(w, r, u.String(), http.StatusFound)
	}
}

func oauthAuthURL(provider string) string {
	switch provider {
	case "google":
		return "https://accounts.google.com/o/oauth2/v2/auth"
	case "github":
		return "https://github.com/login/oauth/authorize"
	default:
		return ""
	}
}

func oauthScope(provider string) string {
	switch provider {
	case "google":
		return "openid email profile"
	case "github":
		return "read:user user:email"
	default:
		return ""
	}
}

func randomURLToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func requestIsSecure(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}
