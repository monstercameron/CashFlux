// SPDX-License-Identifier: MIT

//go:build !js || !wasm

package server

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// sessionCookieSecure defaults to Secure even when a proxy is misconfigured.
// Only an explicitly enabled developer server on a loopback host may opt out.
func sessionCookieSecure(cfg Config, r *http.Request) bool {
	return requestIsSecure(r) || !cfg.DevMode || !sessionLoopbackHost(r.Host)
}

func sessionLoopbackHost(host string) bool {
	u := &url.URL{Host: host}
	name := u.Hostname()
	return strings.EqualFold(name, "localhost") || net.ParseIP(name).IsLoopback()
}

func sessionTransportAllowed(cfg Config, r *http.Request) bool {
	secure := r.TLS != nil || (strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") && ipInNets(remoteHost(r.RemoteAddr), cfg.TrustedProxies))
	return secure || (cfg.DevMode && sessionLoopbackHost(r.Host))
}

// sameSessionOrigin compares the full origin, including scheme and default port.
// Return destinations may include a path; Origin headers are checked separately.
func sameSessionOrigin(raw string, r *http.Request) bool {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Hostname() == "" {
		return false
	}
	scheme := "http"
	if requestIsSecure(r) {
		scheme = "https"
	}
	v := &url.URL{Scheme: scheme, Host: r.Host}
	return strings.EqualFold(u.Scheme, scheme) && strings.EqualFold(u.Hostname(), v.Hostname()) && sessionOriginPort(u) == sessionOriginPort(v)
}

func sessionOriginPort(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	if strings.EqualFold(u.Scheme, "https") {
		return "443"
	}
	return "80"
}

func requireSessionOrigin(w http.ResponseWriter, r *http.Request, cfg Config) bool {
	w.Header().Set("Cache-Control", "no-store")
	u, err := url.Parse(r.Header.Get("Origin"))
	if !sessionTransportAllowed(cfg, r) || err != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || !sameSessionOrigin(r.Header.Get("Origin"), r) {
		writeErrorJSON(w, ErrorReasonPermissionDenied, "secure same-origin request is required")
		return false
	}
	return true
}

func sessionAccessUser(w http.ResponseWriter, r *http.Request, cfg Config, store *Store) (AuthUser, bool) {
	user, ok := httpBearerUser(r, cfg, store)
	if !ok {
		writeErrorJSON(w, ErrorReasonUnauthenticated, "access token is missing or invalid")
		return AuthUser{}, false
	}
	if _, ok := verifySessionClaims(cfg, user.Token, "access", time.Now().UTC()); !ok {
		writeErrorJSON(w, ErrorReasonUnauthenticated, "a rotating session is required")
		return AuthUser{}, false
	}
	if store == nil {
		writeErrorJSON(w, ErrorReasonFailedPrecondition, "store is not configured")
		return AuthUser{}, false
	}
	if suspended, err := store.IsUserSuspended(user.ID); err != nil {
		writeErrorJSON(w, ErrorReasonInternal, "suspension check failed")
		return AuthUser{}, false
	} else if suspended {
		writeErrorJSON(w, ErrorReasonPermissionDenied, "account is suspended")
		return AuthUser{}, false
	}
	return user, true
}

// handleSessionCSRF bootstraps a token-pair session without an existing cookie.
// It requires a bearer access token and an exact same-origin POST. Existing CSRF
// values are preserved so another tab's in-flight refresh binding remains valid.
func handleSessionCSRF(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireSessionOrigin(w, r, cfg) {
			return
		}
		if _, ok := sessionAccessUser(w, r, cfg, store); !ok {
			return
		}
		csrf := ""
		if cookie, err := r.Cookie(sessionCSRFCookie); err == nil {
			csrf = cookie.Value
		}
		if strings.TrimSpace(csrf) == "" {
			var err error
			csrf, err = randomURLToken(32)
			if err != nil {
				writeErrorJSON(w, ErrorReasonInternal, "csrf token issue failed")
				return
			}
		}
		expires := time.Now().Add(sessionRefreshTTL)
		if _, err := oauthDeviceBindingCookie(w, r, sessionCookieSecure(cfg, r), expires); err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "device binding issue failed")
			return
		}
		setCSRFCookieValue(w, csrf, sessionCookieSecure(cfg, r), expires)
		w.Header().Set(sessionCSRFHeader, csrf)
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleSessionCookie converts a password/pairing session to cookie transport.
// The input refresh credential is consumed by the existing bound rotation path;
// its successor is emitted exclusively as an HttpOnly cookie.
func handleSessionCookie(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireSessionOrigin(w, r, cfg) {
			return
		}
		user, ok := sessionAccessUser(w, r, cfg, store)
		if !ok {
			return
		}
		if !validCSRF(r) {
			writeErrorJSON(w, ErrorReasonPermissionDenied, "csrf token is invalid")
			return
		}
		var input struct {
			RefreshToken string `json:"refreshToken"`
		}
		if !decodeSessionRequest(w, r, &input) {
			return
		}
		claims, ok := verifySessionClaims(cfg, input.RefreshToken, "refresh", time.Now().UTC())
		if !ok || claims.Sub != user.ID || claims.JTI == "" || claims.Family == "" {
			writeErrorJSON(w, ErrorReasonUnauthenticated, "refresh token does not belong to this session account")
			return
		}
		// Replace only the request's refresh cookie; preserve CSRF and device binding.
		request := r.Clone(r.Context())
		request.Header.Del("Cookie")
		for _, cookie := range r.Cookies() {
			if cookie.Name != sessionRefreshCookie {
				request.AddCookie(cookie)
			}
		}
		request.AddCookie(&http.Cookie{Name: sessionRefreshCookie, Value: input.RefreshToken})
		handleOAuthRefresh(cfg, store)(w, request)
	}
}

func decodeSessionRequest(w http.ResponseWriter, r *http.Request, output any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	if err := decoder.Decode(output); err != nil {
		writeErrorJSON(w, ErrorReasonInvalidArgument, "session request is invalid")
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeErrorJSON(w, ErrorReasonInvalidArgument, "session request must contain one JSON object")
		return false
	}
	return true
}

func validPKCEChallenge(challenge string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(challenge)
	return err == nil && len(decoded) == 32 && base64.RawURLEncoding.EncodeToString(decoded) == challenge
}

func validPKCEVerifier(verifier string) bool {
	if len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	for _, c := range verifier {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("-._~", c)) {
			return false
		}
	}
	return true
}

func validOAuthCodeReturnTo(raw string, cfg Config) bool {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Host == "" || strings.TrimSpace(cfg.AppOrigin) == "*" {
		return false
	}
	return (u.Scheme == "https" || (cfg.DevMode && u.Scheme == "http" && sessionLoopbackHost(u.Host))) && allowedOrigin(u.Scheme+"://"+u.Host, cfg.AppOrigin)
}

type oauthCodeSessionResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	TokenType    string `json:"tokenType"`
	ExpiresIn    int64  `json:"expiresIn"`
	UserID       string `json:"userId"`
	DeviceID     string `json:"deviceId"`
}

func handleOAuthSessionCode(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if !sessionTransportAllowed(cfg, r) || !writeCORS(w, r, cfg) {
			writeErrorJSON(w, ErrorReasonPermissionDenied, "secure allowed origin is required")
			return
		}
		var input struct {
			Code         string `json:"code"`
			CodeVerifier string `json:"codeVerifier"`
			ReturnTo     string `json:"returnTo"`
		}
		if !decodeSessionRequest(w, r, &input) {
			return
		}
		if len(input.Code) != 43 || !validPKCEVerifier(input.CodeVerifier) || !validOAuthCodeReturnTo(input.ReturnTo, cfg) {
			writeErrorJSON(w, ErrorReasonInvalidArgument, "authorization code request is invalid")
			return
		}
		target, _ := url.Parse(input.ReturnTo)
		if origin := r.Header.Get("Origin"); origin != "" && origin != target.Scheme+"://"+target.Host {
			writeErrorJSON(w, ErrorReasonPermissionDenied, "authorization code origin mismatch")
			return
		}
		if store == nil {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "store is not configured")
			return
		}
		pair, userID, ok, err := store.redeemOAuthSessionCode(cfg, input.Code, input.CodeVerifier, input.ReturnTo, time.Now().UTC())
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "authorization code exchange failed")
			return
		}
		if !ok {
			writeErrorJSON(w, ErrorReasonUnauthenticated, "authorization code is invalid or expired")
			return
		}
		auditFromRequest(r, store, AuthUser{ID: userID}, "auth.code.exchange", "session_family", pair.DeviceID)
		writeJSON(w, oauthCodeSessionResponse{
			AccessToken: pair.AccessToken, RefreshToken: pair.RefreshToken,
			TokenType: "Bearer", ExpiresIn: pair.ExpiresInSeconds, UserID: userID, DeviceID: pair.DeviceID,
		})
	}
}
