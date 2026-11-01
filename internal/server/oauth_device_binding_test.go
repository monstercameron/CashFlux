// SPDX-License-Identifier: MIT

//go:build !js || !wasm

package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOAuthRefreshReplayWithoutDeviceCookieRevokes(t *testing.T) {
	store := openTestStore(t)
	now := time.Now().UTC()
	cfg := Config{AuthMode: "oauth", AppOrigin: "http://127.0.0.1:8080", MasterKey: "0123456789abcdef0123456789abcdef"}
	if err := store.UpsertUser(User{ID: "github:missing-device", Provider: "github", Subject: "missing-device", CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	_, originalRefresh, err := issueStoredSessionPair(cfg, store, "github:missing-device", now, "", "test device")
	if err != nil {
		t.Fatalf("issueStoredSessionPair: %v", err)
	}
	csrfRecorder := httptest.NewRecorder()
	csrf, err := setCSRFCookie(csrfRecorder, false, now.Add(sessionRefreshTTL))
	if err != nil {
		t.Fatalf("setCSRFCookie: %v", err)
	}
	csrfCookie := &http.Cookie{Name: sessionCSRFCookie, Value: csrf}
	deviceRecorder := httptest.NewRecorder()
	deviceID, err := oauthDeviceBindingCookie(deviceRecorder, httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil), false, now.Add(sessionRefreshTTL))
	if err != nil {
		t.Fatalf("oauthDeviceBindingCookie: %v", err)
	}
	deviceCookie := &http.Cookie{Name: sessionDeviceBindingCookie, Value: deviceID}
	handler := handleOAuthRefresh(cfg, store)
	first := oauthRefreshRequest(t, handler, originalRefresh, csrfCookie, deviceCookie)
	if first.Code != http.StatusOK {
		t.Fatalf("first refresh status = %d body %q", first.Code, first.Body.String())
	}
	successor := refreshCookieFromResponse(t, first)

	withoutDevice := oauthRefreshRequest(t, handler, originalRefresh, csrfCookie)
	if withoutDevice.Code != http.StatusUnauthorized {
		t.Fatalf("replay without device cookie status = %d body %q, want 401", withoutDevice.Code, withoutDevice.Body.String())
	}
	assertHTTPErrorReason(t, withoutDevice, ErrorReasonUnauthenticated)
	useSuccessor := oauthRefreshRequest(t, handler, successor.Value, csrfCookie, deviceCookie)
	if useSuccessor.Code != http.StatusUnauthorized {
		t.Fatalf("successor after unbound replay status = %d body %q, want 401", useSuccessor.Code, useSuccessor.Body.String())
	}
}
