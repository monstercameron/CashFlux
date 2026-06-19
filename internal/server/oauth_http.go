package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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
		authURL := oauthAuthURL(providerName, provider)
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

type oauthSessionResponse struct {
	AccessToken string `json:"accessToken"`
	TokenType   string `json:"tokenType"`
	ExpiresIn   int64  `json:"expiresIn"`
	UserID      string `json:"userId"`
}

func handleOAuthCallback(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		providerName := strings.TrimSpace(r.PathValue("provider"))
		provider, ok := cfg.OAuthProviders[providerName]
		if !ok {
			http.Error(w, "oauth provider is not configured", http.StatusNotFound)
			return
		}
		code := strings.TrimSpace(r.URL.Query().Get("code"))
		state := strings.TrimSpace(r.URL.Query().Get("state"))
		if code == "" || state == "" {
			http.Error(w, "oauth code and state are required", http.StatusBadRequest)
			return
		}
		cookie, err := r.Cookie(oauthStateCookie)
		if err != nil {
			http.Error(w, "oauth state cookie is missing", http.StatusBadRequest)
			return
		}
		cookieState, verifier, ok := parseOAuthStateCookie(cookie.Value)
		if !ok || cookieState != state {
			http.Error(w, "oauth state mismatch", http.StatusBadRequest)
			return
		}
		token, err := exchangeOAuthCode(r, providerName, provider, code, verifier)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		user, err := fetchOAuthUser(r, providerName, provider, token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if store == nil {
			http.Error(w, "store is not configured", http.StatusServiceUnavailable)
			return
		}
		if err := store.UpsertUser(user); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		access, refresh, err := issueSessionPair(cfg, user.ID, time.Now().UTC())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		clearOAuthStateCookie(w, providerName, requestIsSecure(r))
		setRefreshCookie(w, refresh, requestIsSecure(r), time.Now().Add(sessionRefreshTTL))
		writeJSON(w, oauthSessionResponse{AccessToken: access, TokenType: "Bearer", ExpiresIn: int64(sessionAccessTTL.Seconds()), UserID: user.ID})
	}
}

func handleOAuthRefresh(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !writeCORS(w, r, cfg) {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
		cookie, err := r.Cookie(sessionRefreshCookie)
		if err != nil {
			http.Error(w, "refresh token is missing", http.StatusUnauthorized)
			return
		}
		userID, ok := verifySessionToken(cfg, cookie.Value, "refresh", time.Now().UTC())
		if !ok {
			http.Error(w, "refresh token is invalid", http.StatusUnauthorized)
			return
		}
		access, refresh, err := issueSessionPair(cfg, userID, time.Now().UTC())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		setRefreshCookie(w, refresh, requestIsSecure(r), time.Now().Add(sessionRefreshTTL))
		writeJSON(w, oauthSessionResponse{AccessToken: access, TokenType: "Bearer", ExpiresIn: int64(sessionAccessTTL.Seconds()), UserID: userID})
	}
}

func handleOAuthLogout(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !writeCORS(w, r, cfg) {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
		setRefreshCookie(w, "", requestIsSecure(r), time.Unix(0, 0))
		w.WriteHeader(http.StatusNoContent)
	}
}

func oauthAuthURL(provider string, cfg OAuthProviderConfig) string {
	if strings.TrimSpace(cfg.AuthURL) != "" {
		return strings.TrimSpace(cfg.AuthURL)
	}
	switch provider {
	case "google":
		return "https://accounts.google.com/o/oauth2/v2/auth"
	case "github":
		return "https://github.com/login/oauth/authorize"
	default:
		return ""
	}
}

func oauthTokenURL(provider string, cfg OAuthProviderConfig) string {
	if strings.TrimSpace(cfg.TokenURL) != "" {
		return strings.TrimSpace(cfg.TokenURL)
	}
	switch provider {
	case "google":
		return "https://oauth2.googleapis.com/token"
	case "github":
		return "https://github.com/login/oauth/access_token"
	default:
		return ""
	}
}

func oauthUserURL(provider string, cfg OAuthProviderConfig) string {
	if strings.TrimSpace(cfg.UserURL) != "" {
		return strings.TrimSpace(cfg.UserURL)
	}
	switch provider {
	case "google":
		return "https://openidconnect.googleapis.com/v1/userinfo"
	case "github":
		return "https://api.github.com/user"
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

func parseOAuthStateCookie(value string) (string, string, bool) {
	state, verifier, ok := strings.Cut(value, ".")
	return state, verifier, ok && state != "" && verifier != ""
}

func exchangeOAuthCode(r *http.Request, providerName string, provider OAuthProviderConfig, code, verifier string) (string, error) {
	endpoint := oauthTokenURL(providerName, provider)
	if endpoint == "" {
		return "", fmt.Errorf("oauth provider is not supported")
	}
	form := url.Values{}
	form.Set("client_id", provider.ClientID)
	form.Set("client_secret", provider.ClientSecret)
	form.Set("code", code)
	form.Set("code_verifier", verifier)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", provider.RedirectURL)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("build oauth token request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("oauth token exchange failed: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("oauth token exchange failed with status %d", resp.StatusCode)
	}
	var body struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return "", fmt.Errorf("parse oauth token response: %w", err)
	}
	if body.Error != "" {
		return "", fmt.Errorf("oauth token exchange failed: %s", body.Error)
	}
	if strings.TrimSpace(body.AccessToken) == "" {
		return "", fmt.Errorf("oauth token response missing access token")
	}
	return body.AccessToken, nil
}

func fetchOAuthUser(r *http.Request, providerName string, provider OAuthProviderConfig, token string) (User, error) {
	endpoint := oauthUserURL(providerName, provider)
	if endpoint == "" {
		return User{}, fmt.Errorf("oauth provider is not supported")
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, endpoint, nil)
	if err != nil {
		return User{}, fmt.Errorf("build oauth user request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return User{}, fmt.Errorf("oauth user fetch failed: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return User{}, fmt.Errorf("oauth user fetch failed with status %d", resp.StatusCode)
	}
	var profile struct {
		ID    any    `json:"id"`
		Sub   string `json:"sub"`
		Email string `json:"email"`
	}
	if err := json.Unmarshal(data, &profile); err != nil {
		return User{}, fmt.Errorf("parse oauth user response: %w", err)
	}
	subject := strings.TrimSpace(profile.Sub)
	if subject == "" {
		subject = strings.TrimSpace(fmt.Sprint(profile.ID))
	}
	if subject == "" || subject == "<nil>" {
		return User{}, fmt.Errorf("oauth user response missing subject")
	}
	return User{
		ID:        providerName + ":" + subject,
		Provider:  providerName,
		Subject:   subject,
		Email:     strings.TrimSpace(profile.Email),
		CreatedAt: time.Now().UTC(),
	}, nil
}

func issueSessionPair(cfg Config, userID string, now time.Time) (string, string, error) {
	access, err := issueSessionToken(cfg, userID, "access", sessionAccessTTL, now)
	if err != nil {
		return "", "", err
	}
	refresh, err := issueSessionToken(cfg, userID, "refresh", sessionRefreshTTL, now)
	if err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

func setRefreshCookie(w http.ResponseWriter, token string, secure bool, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionRefreshCookie,
		Value:    token,
		Path:     "/v1/auth",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
	})
}

func clearOAuthStateCookie(w http.ResponseWriter, providerName string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    "",
		Path:     "/v1/auth/" + providerName + "/callback",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
	})
}
