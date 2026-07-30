// SPDX-License-Identifier: MIT

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeHostedAppFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"index.html":         "<!doctype html><title>CashFlux hosted</title>",
		"services-worker.js": "self.onmessage = function () {};",
		"app.css":            "body { color: green; }",
		"bin/main.wasm":      "\x00asm",
	} {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestHostedAppServesRootDeepLinksAndAssets(t *testing.T) {
	appDir := writeHostedAppFixture(t)
	h := NewMux(Config{AuthMode: "token", AppDir: appDir}, openTestStore(t))

	for _, requestPath := range []string{"/", "/accounts", "/settings/cloud"} {
		t.Run(requestPath, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, requestPath, nil)
			req.Header.Set("Accept", "text/html")
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d body %q", rr.Code, rr.Body.String())
			}
			if !strings.Contains(rr.Body.String(), "CashFlux hosted") {
				t.Fatalf("body = %q, want hosted app shell", rr.Body.String())
			}
			if got := rr.Header().Get("Cache-Control"); got != "no-cache" {
				t.Fatalf("cache-control = %q, want no-cache", got)
			}
			if got := rr.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Fatalf("content-type = %q, want text/html", got)
			}
		})
	}

	req := httptest.NewRequest(http.MethodGet, "/bin/main.wasm", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || rr.Body.String() != "\x00asm" {
		t.Fatalf("wasm status/body = %d %q", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "application/wasm" {
		t.Fatalf("wasm content-type = %q", got)
	}
	if got := rr.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("wasm cache-control = %q", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/app.css", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("css status = %d", rr.Code)
	}
	if got := rr.Header().Get("Cache-Control"); got != "public, max-age=3600" {
		t.Fatalf("css cache-control = %q", got)
	}
}

func TestHostedAppKeepsServerRoutesAndMissingAssetsDistinct(t *testing.T) {
	h := NewMux(Config{AuthMode: "token", AppDir: writeHostedAppFixture(t)}, openTestStore(t))

	req := httptest.NewRequest(http.MethodGet, "/v1/version", nil)
	req.Header.Set("Accept", "text/html")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("version status = %d body %q", rr.Code, rr.Body.String())
	}
	var version VersionResponse
	if err := json.NewDecoder(rr.Body).Decode(&version); err != nil {
		t.Fatalf("decode version: %v", err)
	}
	if !version.HostedApp {
		t.Fatalf("hostedApp = false, want true")
	}

	for _, requestPath := range []string{"/v1/not-a-route", "/grpc/not-a-route", "/missing.js"} {
		t.Run(requestPath, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, requestPath, nil)
			req.Header.Set("Accept", "text/html")
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != http.StatusNotFound {
				t.Fatalf("status = %d body %q, want 404", rr.Code, rr.Body.String())
			}
			if strings.Contains(rr.Body.String(), "CashFlux hosted") {
				t.Fatalf("server/asset miss was masked by SPA fallback: %q", rr.Body.String())
			}
		})
	}

	req = httptest.NewRequest(http.MethodGet, "/../outside.txt", nil)
	rr = httptest.NewRecorder()
	appHandler(Config{AppDir: writeHostedAppFixture(t)}).ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("traversal status = %d body %q, want 404", rr.Code, rr.Body.String())
	}
}

func TestHostedRootStillProvidesJSONDiscovery(t *testing.T) {
	h := NewMux(Config{AuthMode: "token", AppDir: writeHostedAppFixture(t)}, openTestStore(t))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body %q", rr.Code, rr.Body.String())
	}
	var body RootResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode root: %v", err)
	}
	if body.Service != "cashflux-server" {
		t.Fatalf("service = %q", body.Service)
	}
}

func TestAppDirConfiguration(t *testing.T) {
	t.Setenv("CASHFLUX_SERVER_APP_DIR", "dist/cashflux")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	if cfg.AppDir != "dist/cashflux" {
		t.Fatalf("AppDir = %q", cfg.AppDir)
	}
}
