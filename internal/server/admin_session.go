// SPDX-License-Identifier: MIT

package server

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	adminAccessCookie  = "cashflux_admin_access"
	adminRefreshCookie = "cashflux_admin_refresh"
	adminCSRFCookie    = "cashflux_admin_csrf"
	adminCookiePath    = "/v1"
	adminRefreshPath   = "/v1/admin/session"
)

type adminSessionLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type adminSessionResponse struct {
	Authenticated bool   `json:"authenticated"`
	UserID        string `json:"userId"`
	Username      string `json:"username,omitempty"`
	Role          string `json:"role"`
	ExpiresIn     int64  `json:"expiresIn"`
}

// adminAuthSource distinguishes an explicit bearer credential from the
// browser-only HttpOnly operator cookie. Only the latter needs the admin CSRF
// proof because bearer requests are not ambient browser authority.
type adminAuthSource uint8

const (
	adminAuthNone adminAuthSource = iota
	adminAuthBearer
	adminAuthCookie
)

func handleAdminSessionLogin(cfg Config, store *Store, auth *authServer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !adminSessionOriginAllowed(w, r, cfg) {
			return
		}
		if store == nil || auth == nil {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "store is not configured")
			return
		}
		var req adminSessionLoginRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
			writeErrorJSON(w, ErrorReasonInvalidArgument, "invalid request body")
			return
		}
		out, err := auth.Login(r.Context(), backendrpc.LoginRequest{
			Username:    strings.TrimSpace(req.Username),
			Password:    req.Password,
			DeviceLabel: "CashFlux operator console",
		})
		if err != nil {
			writeAdminSessionError(w, err)
			return
		}
		userID, ok := verifySessionToken(cfg, out.AccessToken, "access", time.Now().UTC())
		if !ok {
			_ = store.RevokeRefreshSessionFamily(out.DeviceID, time.Now().UTC())
			writeErrorJSON(w, ErrorReasonInternal, "session verification failed")
			return
		}
		user := AuthUser{ID: userID, Token: out.AccessToken}
		allowed, role, authErr := adminOperatorAuthorized(user, cfg, store)
		if authErr != nil {
			_ = store.RevokeRefreshSessionFamily(out.DeviceID, time.Now().UTC())
			writeErrorJSON(w, ErrorReasonInternal, "role lookup failed")
			return
		}
		if !allowed {
			_ = store.RevokeRefreshSessionFamily(out.DeviceID, time.Now().UTC())
			auditFromRequest(r, store, user, "admin.session.login.denied", "admin", "session")
			clearAdminSessionCookies(w, requestIsSecure(r))
			writeErrorJSON(w, ErrorReasonPermissionDenied, "owner access required")
			return
		}
		record, found, err := store.GetUserByID(userID)
		if err != nil || !found {
			_ = store.RevokeRefreshSessionFamily(out.DeviceID, time.Now().UTC())
			writeErrorJSON(w, ErrorReasonInternal, "account lookup failed")
			return
		}
		csrf, err := setAdminSessionCookies(w, out.AccessToken, out.RefreshToken, requestIsSecure(r), time.Now().UTC())
		if err != nil {
			_ = store.RevokeRefreshSessionFamily(out.DeviceID, time.Now().UTC())
			writeErrorJSON(w, ErrorReasonInternal, "csrf cookie issue failed")
			return
		}
		w.Header().Set(sessionCSRFHeader, csrf)
		auditFromRequest(r, store, user, "admin.session.login", "admin", "session")
		writeJSON(w, adminSessionResponse{
			Authenticated: true,
			UserID:        userID,
			Username:      record.Username,
			Role:          role,
			ExpiresIn:     int64(sessionAccessTTL.Seconds()),
		})
	}
}

func handleAdminSessionStatus(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !adminSessionOriginAllowed(w, r, cfg) {
			return
		}
		if store == nil {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "store is not configured")
			return
		}
		user, ok := httpAdminCookieUser(r, cfg, store)
		if !ok {
			writeErrorJSON(w, ErrorReasonUnauthenticated, "operator session is missing or expired")
			return
		}
		allowed, role, err := adminOperatorAuthorized(user, cfg, store)
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "role lookup failed")
			return
		}
		if !allowed {
			clearAdminSessionCookies(w, requestIsSecure(r))
			writeErrorJSON(w, ErrorReasonPermissionDenied, "owner access required")
			return
		}
		record, found, err := store.GetUserByID(user.ID)
		if err != nil || !found {
			writeErrorJSON(w, ErrorReasonUnauthenticated, "operator account is unavailable")
			return
		}
		csrf := adminCSRFToken(r)
		if csrf == "" {
			csrf, err = setAdminCSRFCookie(w, requestIsSecure(r), time.Now().UTC().Add(sessionRefreshTTL))
			if err != nil {
				writeErrorJSON(w, ErrorReasonInternal, "csrf cookie issue failed")
				return
			}
		}
		w.Header().Set(sessionCSRFHeader, csrf)
		writeJSON(w, adminSessionResponse{
			Authenticated: true,
			UserID:        user.ID,
			Username:      record.Username,
			Role:          role,
			ExpiresIn:     int64(sessionAccessTTL.Seconds()),
		})
	}
}

func handleAdminSessionRefresh(cfg Config, store *Store, auth *authServer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !adminSessionOriginAllowed(w, r, cfg) {
			return
		}
		if !validAdminCSRF(r) {
			writeErrorJSON(w, ErrorReasonPermissionDenied, "csrf token is invalid")
			return
		}
		if store == nil || auth == nil {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "store is not configured")
			return
		}
		cookie, err := r.Cookie(adminRefreshCookie)
		if err != nil || strings.TrimSpace(cookie.Value) == "" {
			writeErrorJSON(w, ErrorReasonUnauthenticated, "refresh token is missing")
			return
		}
		out, err := auth.RefreshToken(r.Context(), backendrpc.RefreshTokenRequest{RefreshToken: cookie.Value})
		if err != nil {
			clearAdminSessionCookies(w, requestIsSecure(r))
			writeAdminSessionError(w, err)
			return
		}
		userID, ok := verifySessionToken(cfg, out.AccessToken, "access", time.Now().UTC())
		if !ok {
			_ = store.RevokeRefreshSessionFamily(out.DeviceID, time.Now().UTC())
			clearAdminSessionCookies(w, requestIsSecure(r))
			writeErrorJSON(w, ErrorReasonInternal, "session verification failed")
			return
		}
		user := AuthUser{ID: userID, Token: out.AccessToken}
		allowed, role, authErr := adminOperatorAuthorized(user, cfg, store)
		if authErr != nil {
			_ = store.RevokeRefreshSessionFamily(out.DeviceID, time.Now().UTC())
			clearAdminSessionCookies(w, requestIsSecure(r))
			writeErrorJSON(w, ErrorReasonInternal, "role lookup failed")
			return
		}
		if !allowed {
			_ = store.RevokeRefreshSessionFamily(out.DeviceID, time.Now().UTC())
			clearAdminSessionCookies(w, requestIsSecure(r))
			writeErrorJSON(w, ErrorReasonPermissionDenied, "owner access required")
			return
		}
		record, found, err := store.GetUserByID(userID)
		if err != nil || !found {
			_ = store.RevokeRefreshSessionFamily(out.DeviceID, time.Now().UTC())
			clearAdminSessionCookies(w, requestIsSecure(r))
			writeErrorJSON(w, ErrorReasonUnauthenticated, "operator account is unavailable")
			return
		}
		csrf, err := setAdminSessionCookies(w, out.AccessToken, out.RefreshToken, requestIsSecure(r), time.Now().UTC())
		if err != nil {
			_ = store.RevokeRefreshSessionFamily(out.DeviceID, time.Now().UTC())
			writeErrorJSON(w, ErrorReasonInternal, "csrf cookie issue failed")
			return
		}
		w.Header().Set(sessionCSRFHeader, csrf)
		writeJSON(w, adminSessionResponse{
			Authenticated: true,
			UserID:        userID,
			Username:      record.Username,
			Role:          role,
			ExpiresIn:     int64(sessionAccessTTL.Seconds()),
		})
	}
}

func handleAdminSessionLogout(cfg Config, store *Store, auth *authServer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !adminSessionOriginAllowed(w, r, cfg) {
			return
		}
		if !validAdminCSRF(r) {
			writeErrorJSON(w, ErrorReasonPermissionDenied, "csrf token is invalid")
			return
		}
		if cookie, err := r.Cookie(adminRefreshCookie); err == nil && auth != nil {
			_, _ = auth.Logout(r.Context(), backendrpc.LogoutRequest{RefreshToken: cookie.Value})
		}
		if user, ok := httpAdminCookieUser(r, cfg, store); ok {
			auditFromRequest(r, store, user, "admin.session.logout", "admin", "session")
		}
		clearAdminSessionCookies(w, requestIsSecure(r))
		w.WriteHeader(http.StatusNoContent)
	}
}

func adminSessionOriginAllowed(w http.ResponseWriter, r *http.Request, cfg Config) bool {
	if !writeCORS(w, r, cfg) {
		writeErrorJSON(w, ErrorReasonPermissionDenied, "origin not allowed")
		return false
	}
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" && !sameRequestOrigin(origin, r) {
		writeErrorJSON(w, ErrorReasonPermissionDenied, "operator sessions require the console origin")
		return false
	}
	return true
}

func setAdminSessionCookies(w http.ResponseWriter, access, refresh string, secure bool, now time.Time) (string, error) {
	http.SetCookie(w, &http.Cookie{
		Name:     adminAccessCookie,
		Value:    access,
		Path:     adminCookiePath,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		Expires:  now.Add(sessionAccessTTL),
	})
	http.SetCookie(w, &http.Cookie{
		Name:     adminRefreshCookie,
		Value:    refresh,
		Path:     adminRefreshPath,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		Expires:  now.Add(sessionRefreshTTL),
	})
	return setAdminCSRFCookie(w, secure, now.Add(sessionRefreshTTL))
}

func setAdminCSRFCookie(w http.ResponseWriter, secure bool, expires time.Time) (string, error) {
	token, err := randomURLToken(32)
	if err != nil {
		return "", err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     adminCSRFCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		Expires:  expires,
	})
	return token, nil
}

func clearAdminSessionCookies(w http.ResponseWriter, secure bool) {
	for _, cookie := range []*http.Cookie{
		{Name: adminAccessCookie, Path: adminCookiePath, HttpOnly: true},
		{Name: adminRefreshCookie, Path: adminRefreshPath, HttpOnly: true},
		{Name: adminCSRFCookie, Path: "/"},
	} {
		cookie.Value = ""
		cookie.Secure = secure
		cookie.SameSite = http.SameSiteStrictMode
		cookie.Expires = time.Unix(0, 0)
		cookie.MaxAge = -1
		http.SetCookie(w, cookie)
	}
}

func httpAdminCookieUser(r *http.Request, cfg Config, store *Store) (AuthUser, bool) {
	cookie, err := r.Cookie(adminAccessCookie)
	if err != nil {
		return AuthUser{}, false
	}
	userID, ok := verifySessionToken(cfg, cookie.Value, "access", time.Now().UTC())
	if !ok {
		return AuthUser{}, false
	}
	// Same rule as the bearer path: an operator cookie minted before the account
	// was deleted stays cryptographically valid for its full 15 minutes. It
	// matters most here, because an id named in cfg.AdminUserIDs is granted
	// owner authority from configuration alone — deleting the account would not
	// otherwise have taken that away until the cookie expired.
	if store != nil {
		if deleted, err := store.AccountDeleted(userID); err != nil || deleted {
			return AuthUser{}, false
		}
	}
	return AuthUser{ID: userID, Token: cookie.Value}, true
}

func adminRequestUser(r *http.Request, cfg Config, store *Store) (AuthUser, adminAuthSource, bool) {
	if user, ok := httpBearerUser(r, cfg, store); ok {
		return user, adminAuthBearer, true
	}
	if user, ok := httpAdminCookieUser(r, cfg, store); ok {
		return user, adminAuthCookie, true
	}
	return AuthUser{}, adminAuthNone, false
}

func adminOperatorAuthorized(user AuthUser, cfg Config, store *Store) (bool, string, error) {
	if cfg.matchesStaticToken(user.Token) {
		return true, RoleOwner, nil
	}
	if cfg.IsAdmin(user.ID) {
		role, err := store.UserRole(user.ID)
		if err != nil {
			return false, "", err
		}
		if strings.TrimSpace(role) == "" {
			role = RoleOwner
		}
		return true, role, nil
	}
	role, err := store.UserRole(user.ID)
	if err != nil {
		return false, "", err
	}
	return role == RoleOwner, role, nil
}

func adminCSRFToken(r *http.Request) string {
	cookie, err := r.Cookie(adminCSRFCookie)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func validAdminCSRF(r *http.Request) bool {
	cookie := adminCSRFToken(r)
	header := strings.TrimSpace(r.Header.Get(sessionCSRFHeader))
	return cookie != "" && header != "" &&
		subtle.ConstantTimeCompare([]byte(cookie), []byte(header)) == 1
}

func writeAdminSessionError(w http.ResponseWriter, err error) {
	switch status.Code(err) {
	case codes.InvalidArgument:
		writeErrorJSON(w, ErrorReasonInvalidArgument, status.Convert(err).Message())
	case codes.Unauthenticated:
		writeErrorJSON(w, ErrorReasonUnauthenticated, status.Convert(err).Message())
	case codes.PermissionDenied:
		writeErrorJSON(w, ErrorReasonPermissionDenied, status.Convert(err).Message())
	case codes.ResourceExhausted:
		writeErrorJSON(w, ErrorReasonRateLimited, status.Convert(err).Message())
	case codes.FailedPrecondition:
		writeErrorJSON(w, ErrorReasonFailedPrecondition, status.Convert(err).Message())
	default:
		writeErrorJSON(w, ErrorReasonInternal, "operator session failed")
	}
}
