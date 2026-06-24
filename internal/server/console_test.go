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

func TestConsoleHandlerServesIndexHTML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><html><body>console</body></html>"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := Config{AuthMode: "token", ConsoleDir: dir}
	h := NewMux(cfg, openTestStore(t))
	req := httptest.NewRequest(http.MethodGet, "/console/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body %q", rr.Code, rr.Body.String())
	}
	if !strings.Contains(strings.ToLower(rr.Body.String()), "<!doctype html") {
		t.Fatalf("body missing doctype: %q", rr.Body.String())
	}
}

func TestConsoleHandlerSPAFallback(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><html><body>console</body></html>"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := Config{AuthMode: "token", ConsoleDir: dir}
	h := NewMux(cfg, openTestStore(t))
	req := httptest.NewRequest(http.MethodGet, "/console/some-unknown-path", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body %q", rr.Code, rr.Body.String())
	}
	if !strings.Contains(strings.ToLower(rr.Body.String()), "<!doctype html") {
		t.Fatalf("body missing doctype: %q", rr.Body.String())
	}
}

func TestConsoleRedirectNoTrailingSlash(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><html><body>console</body></html>"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := Config{AuthMode: "token", ConsoleDir: dir}
	h := NewMux(cfg, openTestStore(t))
	req := httptest.NewRequest(http.MethodGet, "/console", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d body %q, want 302", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Location"); got != "/console/" {
		t.Fatalf("Location = %q, want /console/", got)
	}
}

func TestRootRedirectsToConsoleForBrowser(t *testing.T) {
	h := NewMux(Config{AuthMode: "token"}, openTestStore(t))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d body %q, want 302", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Location"); got != "/console/" {
		t.Fatalf("Location = %q, want /console/", got)
	}
}

func TestRootReturnsJSONWithoutAcceptHeader(t *testing.T) {
	h := NewMux(Config{AuthMode: "token"}, openTestStore(t))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
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
		t.Fatalf("service = %q, want cashflux-server", body.Service)
	}
}

func TestRootReturnsJSONWithJSONAcceptHeader(t *testing.T) {
	h := NewMux(Config{AuthMode: "token"}, openTestStore(t))
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
		t.Fatalf("service = %q, want cashflux-server", body.Service)
	}
}
