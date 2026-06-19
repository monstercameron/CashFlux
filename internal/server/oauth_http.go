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
const oauthIDTokenClockSkew = 5 * time.Minute

func handleOAuthStart(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		providerName := strings.TrimSpace(r.PathValue("provider"))
		provider, ok := cfg.OAuthProviders[providerName]
		if !ok {
			writeErrorJSON(w, ErrorReasonNotFound, "oauth provider is not configured")
			return
		}
		authURL := oauthAuthURL(providerName, provider)
		if authURL == "" {
			writeErrorJSON(w, ErrorReasonNotFound, "oauth provider is not supported")
			return
		}
		state, err := randomURLToken(32)
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "build oauth state")
			return
		}
		verifier, err := randomURLToken(48)
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "build oauth verifier")
			return
		}
		nonce, err := randomURLToken(32)
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "build oauth nonce")
			return
		}
		challenge := pkceChallenge(verifier)
		http.SetCookie(w, &http.Cookie{
			Name:     oauthStateCookie,
			Value:    state + "." + verifier + "." + nonce,
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
		if providerName == "google" {
			q.Set("nonce", nonce)
		}
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
			writeErrorJSON(w, ErrorReasonNotFound, "oauth provider is not configured")
			return
		}
		code := strings.TrimSpace(r.URL.Query().Get("code"))
		state := strings.TrimSpace(r.URL.Query().Get("state"))
		if code == "" || state == "" {
			writeErrorJSON(w, ErrorReasonInvalidArgument, "oauth code and state are required")
			return
		}
		cookie, err := r.Cookie(oauthStateCookie)
		if err != nil {
			writeErrorJSON(w, ErrorReasonInvalidArgument, "oauth state cookie is missing")
			return
		}
		cookieState, verifier, nonce, ok := parseOAuthStateCookie(cookie.Value)
		if !ok || cookieState != state {
			writeErrorJSON(w, ErrorReasonInvalidArgument, "oauth state mismatch")
			return
		}
		token, err := exchangeOAuthCode(r, providerName, provider, code, verifier)
		if err != nil {
			writeErrorJSON(w, ErrorReasonUpstreamUnavailable, err.Error())
			return
		}
		if err := validateOAuthIDToken(providerName, provider, token.IDToken, nonce); err != nil {
			writeErrorJSON(w, ErrorReasonUpstreamUnavailable, err.Error())
			return
		}
		user, err := fetchOAuthUser(r, providerName, provider, token.AccessToken)
		if err != nil {
			writeErrorJSON(w, ErrorReasonUpstreamUnavailable, err.Error())
			return
		}
		if store == nil {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "store is not configured")
			return
		}
		if err := store.UpsertUser(user); err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "user upsert failed")
			return
		}
		now := time.Now().UTC()
		auditFromRequest(r, store, AuthUser{ID: user.ID}, "auth.login", "user", user.ID)
		access, refresh, err := issueStoredSessionPair(cfg, store, user.ID, now, "")
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "session issue failed")
			return
		}
		clearOAuthStateCookie(w, providerName, requestIsSecure(r))
		setRefreshCookie(w, refresh, requestIsSecure(r), now.Add(sessionRefreshTTL))
		csrf, err := setCSRFCookie(w, requestIsSecure(r), now.Add(sessionRefreshTTL))
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "csrf cookie issue failed")
			return
		}
		resp := oauthSessionResponse{AccessToken: access, TokenType: "Bearer", ExpiresIn: int64(sessionAccessTTL.Seconds()), UserID: user.ID}
		w.Header().Set(sessionCSRFHeader, csrf)
		writeJSON(w, resp)
	}
}

func handleOAuthRefresh(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !writeCORS(w, r, cfg) {
			writeErrorJSON(w, ErrorReasonPermissionDenied, "origin not allowed")
			return
		}
		if !validCSRF(r) {
			writeErrorJSON(w, ErrorReasonPermissionDenied, "csrf token is invalid")
			return
		}
		cookie, err := r.Cookie(sessionRefreshCookie)
		if err != nil {
			writeErrorJSON(w, ErrorReasonUnauthenticated, "refresh token is missing")
			return
		}
		if store == nil {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "store is not configured")
			return
		}
		now := time.Now().UTC()
		claims, ok := verifySessionClaims(cfg, cookie.Value, "refresh", now)
		if !ok {
			writeErrorJSON(w, ErrorReasonUnauthenticated, "refresh token is invalid")
			return
		}
		if strings.TrimSpace(claims.JTI) == "" || strings.TrimSpace(claims.Family) == "" {
			writeErrorJSON(w, ErrorReasonUnauthenticated, "refresh token is invalid")
			return
		}
		session, ok, err := store.ConsumeRefreshSession(claims.JTI, sessionTokenHash(cookie.Value), now)
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "refresh token lookup failed")
			return
		}
		if !ok {
			if strings.TrimSpace(session.FamilyID) != "" {
				_ = store.RevokeRefreshSessionFamily(session.FamilyID, now)
				auditFromRequest(r, store, AuthUser{ID: session.UserID}, "auth.token.reuse", "session_family", session.FamilyID)
			}
			writeErrorJSON(w, ErrorReasonUnauthenticated, "refresh token is invalid")
			return
		}
		access, refresh, err := issueStoredSessionPair(cfg, store, session.UserID, now, session.FamilyID)
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "session issue failed")
			return
		}
		auditFromRequest(r, store, AuthUser{ID: session.UserID}, "auth.token.refresh", "user", session.UserID)
		setRefreshCookie(w, refresh, requestIsSecure(r), now.Add(sessionRefreshTTL))
		csrf, err := setCSRFCookie(w, requestIsSecure(r), now.Add(sessionRefreshTTL))
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "csrf cookie issue failed")
			return
		}
		w.Header().Set(sessionCSRFHeader, csrf)
		writeJSON(w, oauthSessionResponse{AccessToken: access, TokenType: "Bearer", ExpiresIn: int64(sessionAccessTTL.Seconds()), UserID: session.UserID})
	}
}

func handleOAuthLogout(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !writeCORS(w, r, cfg) {
			writeErrorJSON(w, ErrorReasonPermissionDenied, "origin not allowed")
			return
		}
		if !validCSRF(r) {
			writeErrorJSON(w, ErrorReasonPermissionDenied, "csrf token is invalid")
			return
		}
		now := time.Now().UTC()
		if cookie, err := r.Cookie(sessionRefreshCookie); err == nil {
			if claims, ok := verifySessionClaims(cfg, cookie.Value, "refresh", now); ok {
				if store != nil && strings.TrimSpace(claims.Family) != "" {
					_ = store.RevokeRefreshSessionFamily(claims.Family, now)
				}
				auditFromRequest(r, store, AuthUser{ID: claims.Sub}, "auth.logout", "user", claims.Sub)
			}
		}
		setRefreshCookie(w, "", requestIsSecure(r), time.Unix(0, 0))
		setExpiredCSRFCookie(w, requestIsSecure(r))
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleOAuthLogoutAll(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !writeCORS(w, r, cfg) {
			writeErrorJSON(w, ErrorReasonPermissionDenied, "origin not allowed")
			return
		}
		if !validCSRF(r) {
			writeErrorJSON(w, ErrorReasonPermissionDenied, "csrf token is invalid")
			return
		}
		user, ok := httpBearerUser(r, cfg)
		if !ok {
			writeErrorJSON(w, ErrorReasonUnauthenticated, "access token is missing or invalid")
			return
		}
		if store == nil {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "store is not configured")
			return
		}
		now := time.Now().UTC()
		if err := store.RevokeRefreshSessionsForUser(user.ID, now); err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "session revoke failed")
			return
		}
		auditFromRequest(r, store, user, "auth.logout_all", "user", user.ID)
		setRefreshCookie(w, "", requestIsSecure(r), time.Unix(0, 0))
		setExpiredCSRFCookie(w, requestIsSecure(r))
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

func parseOAuthStateCookie(value string) (string, string, string, bool) {
	state, rest, ok := strings.Cut(value, ".")
	if !ok {
		return "", "", "", false
	}
	verifier, nonce, ok := strings.Cut(rest, ".")
	return state, verifier, nonce, ok && state != "" && verifier != "" && nonce != ""
}

type oauthToken struct {
	AccessToken string
	IDToken     string
}

func exchangeOAuthCode(r *http.Request, providerName string, provider OAuthProviderConfig, code, verifier string) (oauthToken, error) {
	endpoint := oauthTokenURL(providerName, provider)
	if endpoint == "" {
		return oauthToken{}, fmt.Errorf("oauth provider is not supported")
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
		return oauthToken{}, fmt.Errorf("build oauth token request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return oauthToken{}, fmt.Errorf("oauth token exchange failed: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return oauthToken{}, fmt.Errorf("oauth token exchange failed with status %d", resp.StatusCode)
	}
	var body struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
		TokenType   string `json:"token_type"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return oauthToken{}, fmt.Errorf("parse oauth token response: %w", err)
	}
	if body.Error != "" {
		return oauthToken{}, fmt.Errorf("oauth token exchange failed: %s", body.Error)
	}
	if strings.TrimSpace(body.AccessToken) == "" {
		return oauthToken{}, fmt.Errorf("oauth token response missing access token")
	}
	return oauthToken{AccessToken: strings.TrimSpace(body.AccessToken), IDToken: strings.TrimSpace(body.IDToken)}, nil
}

func validateOAuthIDToken(providerName string, provider OAuthProviderConfig, rawIDToken, nonce string) error {
	rawIDToken = strings.TrimSpace(rawIDToken)
	if rawIDToken == "" {
		if providerName == "google" {
			return fmt.Errorf("oauth id token is required")
		}
		return nil
	}
	parts := strings.Split(rawIDToken, ".")
	if len(parts) != 3 {
		return fmt.Errorf("oauth id token is malformed")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("oauth id token payload is malformed")
	}
	var claims struct {
		Issuer   string `json:"iss"`
		Audience any    `json:"aud"`
		Nonce    string `json:"nonce"`
		Expires  int64  `json:"exp"`
		IssuedAt int64  `json:"iat"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return fmt.Errorf("parse oauth id token claims: %w", err)
	}
	if !validOAuthIssuer(providerName, claims.Issuer) {
		return fmt.Errorf("oauth id token issuer is invalid")
	}
	if !oauthAudienceContains(claims.Audience, provider.ClientID) {
		return fmt.Errorf("oauth id token audience is invalid")
	}
	if providerName == "google" && strings.TrimSpace(claims.Nonce) != nonce {
		return fmt.Errorf("oauth id token nonce is invalid")
	}
	now := time.Now().UTC()
	if claims.Expires <= 0 {
		return fmt.Errorf("oauth id token expiry is missing")
	}
	if now.After(time.Unix(claims.Expires, 0).Add(oauthIDTokenClockSkew)) {
		return fmt.Errorf("oauth id token is expired")
	}
	if claims.IssuedAt > 0 && time.Unix(claims.IssuedAt, 0).After(now.Add(oauthIDTokenClockSkew)) {
		return fmt.Errorf("oauth id token issued-at is invalid")
	}
	return nil
}

func validOAuthIssuer(providerName, issuer string) bool {
	issuer = strings.TrimSpace(issuer)
	switch providerName {
	case "google":
		return issuer == "https://accounts.google.com" || issuer == "accounts.google.com"
	default:
		return issuer != ""
	}
}

func oauthAudienceContains(raw any, clientID string) bool {
	clientID = strings.TrimSpace(clientID)
	switch aud := raw.(type) {
	case string:
		return strings.TrimSpace(aud) == clientID
	case []any:
		for _, v := range aud {
			if s, ok := v.(string); ok && strings.TrimSpace(s) == clientID {
				return true
			}
		}
	}
	return false
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

func issueStoredSessionPair(cfg Config, store *Store, userID string, now time.Time, familyID string) (string, string, error) {
	if store == nil {
		return "", "", fmt.Errorf("store is not configured")
	}
	familyID = strings.TrimSpace(familyID)
	if familyID == "" {
		var err error
		familyID, err = randomURLToken(24)
		if err != nil {
			return "", "", fmt.Errorf("server session: generate refresh family: %w", err)
		}
	}
	jti, err := randomURLToken(24)
	if err != nil {
		return "", "", fmt.Errorf("server session: generate refresh jti: %w", err)
	}
	access, err := issueSessionToken(cfg, userID, "access", sessionAccessTTL, now)
	if err != nil {
		return "", "", err
	}
	refresh, err := issueSessionTokenWithClaims(cfg, sessionClaims{
		Sub:    userID,
		Type:   "refresh",
		Exp:    now.Add(sessionRefreshTTL).Unix(),
		JTI:    jti,
		Family: familyID,
	})
	if err != nil {
		return "", "", err
	}
	if err := store.PutRefreshSession(RefreshSession{
		JTI:       jti,
		FamilyID:  familyID,
		UserID:    userID,
		TokenHash: sessionTokenHash(refresh),
		ExpiresAt: now.Add(sessionRefreshTTL),
	}); err != nil {
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

func setCSRFCookie(w http.ResponseWriter, secure bool, expires time.Time) (string, error) {
	token, err := randomURLToken(32)
	if err != nil {
		return "", fmt.Errorf("server session: generate csrf token: %w", err)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCSRFCookie,
		Value:    token,
		Path:     "/v1/auth",
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		Expires:  expires,
	})
	return token, nil
}

func setExpiredCSRFCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCSRFCookie,
		Value:    "",
		Path:     "/v1/auth",
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Unix(0, 0),
	})
}

func validCSRF(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCSRFCookie)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return false
	}
	return strings.TrimSpace(r.Header.Get(sessionCSRFHeader)) == cookie.Value
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
