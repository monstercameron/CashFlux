// SPDX-License-Identifier: MIT

//go:build !js || !wasm

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSyncBridgeHandlersResolveMe(t *testing.T) {
	tests := []struct {
		name    string
		handler func(Config, ...*Store) http.Handler
	}{
		{name: "sync", handler: NewSyncBridgeHandler},
		{name: "sync and auth", handler: NewSyncAndAuthBridgeHandler},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{AuthMode: "token", Token: "bridge-me-test-token", AppOrigin: "*"}
			req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
			req.Header.Set("Authorization", "Bearer "+cfg.Token)
			rr := httptest.NewRecorder()
			tt.handler(cfg).ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("GET /v1/me status = %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
			}
			var got struct {
				UserID  string `json:"userId"`
				Billing struct {
					Enabled bool `json:"enabled"`
				} `json:"billing"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode /v1/me response: %v", err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(rr.Body.Bytes(), &fields); err != nil {
				t.Fatalf("decode /v1/me fields: %v", err)
			}
			if len(fields) != 1 {
				t.Errorf("/v1/me returned fields %v, want only userId when no store is available", fields)
			}
			want := authUserFromToken(cfg.Token).ID
			if got.UserID != want || got.UserID == "" {
				t.Errorf("userId = %q, want %q", got.UserID, want)
			}
			if got.Billing.Enabled {
				t.Error("billing.enabled = true, want false on embedded sync bridge")
			}
		})
	}
}

func TestSyncBridgeHandlersMePreflight(t *testing.T) {
	tests := []struct {
		name    string
		handler func(Config, ...*Store) http.Handler
	}{
		{name: "sync", handler: NewSyncBridgeHandler},
		{name: "sync and auth", handler: NewSyncAndAuthBridgeHandler},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{AuthMode: "token", Token: "bridge-me-test-token", AppOrigin: "*"}
			req := httptest.NewRequest(http.MethodOptions, "/v1/me", nil)
			req.Header.Set("Origin", "https://cashflux.example")
			rr := httptest.NewRecorder()
			tt.handler(cfg).ServeHTTP(rr, req)

			if rr.Code != http.StatusNoContent {
				t.Fatalf("OPTIONS /v1/me status = %d, want %d", rr.Code, http.StatusNoContent)
			}
			if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "*" {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "*")
			}
		})
	}
}
