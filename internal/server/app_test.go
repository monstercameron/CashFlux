// SPDX-License-Identifier: MIT

//go:build !js || !wasm

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
		"index.html":         "<!doctype html><html><head><title>CashFlux hosted</title></head></html>",
		"build.json":         `{"buildId":"v1.20.0-abc1234","schema":7}`,
		"favicon.ico":        "ico fixture bytes",
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

func TestHostedAppServesFaviconICOWithExplicitTypeAndCache(t *testing.T) {
	appDir := writeHostedAppFixture(t)
	h := appHandler(Config{AppDir: appDir})
	req := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || rr.Body.String() != "ico fixture bytes" {
		t.Fatalf("favicon response = %d %q", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "image/x-icon" {
		t.Errorf("Content-Type = %q, want image/x-icon", got)
	}
	if got := rr.Header().Get("Cache-Control"); got != "public, max-age=3600" {
		t.Errorf("Cache-Control = %q, want cacheable favicon", got)
	}

	missingDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(missingDir, "index.html"), []byte("<html><head></head></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := httptest.NewRecorder()
	appHandler(Config{AppDir: missingDir}).ServeHTTP(missing, req)
	if missing.Code != http.StatusNotFound || strings.Contains(missing.Body.String(), "<html>") {
		t.Fatalf("missing favicon fell through to SPA: %d %q", missing.Code, missing.Body.String())
	}
}

func TestStaticAndServerHTMLSurfacesHaveCashFluxIcons(t *testing.T) {
	for _, name := range []string{"index.html", "admin/index.html", "portal/index.html"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "web", filepath.FromSlash(name)))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		assertCashFluxIconLinks(t, name, string(data))
	}
	target := "https://cashflux.example/callback"
	rr := httptest.NewRecorder()
	writeOAuthCallbackHTML(rr, target, oauthSessionResponse{AccessToken: "token", TokenType: "Bearer", ExpiresIn: 60, UserID: "user"}, "csrf")
	assertCashFluxIconLinks(t, "OAuth callback", rr.Body.String())
}

func TestManifestSeparatesAnyAndMaskableIcons(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "web", "manifest.webmanifest"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Icons []struct {
			Src     string `json:"src"`
			Purpose string `json:"purpose"`
		} `json:"icons"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	want := map[string]string{
		"./icon-192.png":          "any",
		"./icon-512.png":          "any",
		"./icon-maskable-192.png": "maskable",
		"./icon-maskable-512.png": "maskable",
	}
	for _, icon := range manifest.Icons {
		if purpose, ok := want[icon.Src]; ok {
			if icon.Purpose != purpose {
				t.Errorf("%s purpose = %q, want %q", icon.Src, icon.Purpose, purpose)
			}
			delete(want, icon.Src)
		}
	}
	for missing := range want {
		t.Errorf("manifest missing %s", missing)
	}
}

func assertCashFluxIconLinks(t *testing.T, surface, body string) {
	t.Helper()
	for _, link := range []string{
		`href="/favicon.ico" sizes="48x48"`,
		`type="image/svg+xml" href="/favicon.svg"`,
		`type="image/png" sizes="32x32" href="/favicon-32.png"`,
		`rel="apple-touch-icon" href="/apple-touch-icon.png"`,
		`rel="mask-icon" href="/favicon.svg" color="#2e8b57"`,
		`name="theme-color" content="#2e8b57"`,
	} {
		if !strings.Contains(body, link) {
			t.Errorf("%s missing icon markup %q", surface, link)
		}
	}
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
			if !strings.Contains(rr.Body.String(), hostedAppMeta) {
				t.Fatalf("body = %q, want hosted-app marker", rr.Body.String())
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

func TestHostedAppServesBuildJSONWithoutCaching(t *testing.T) {
	h := NewMux(Config{AuthMode: "token", AppDir: writeHostedAppFixture(t)}, openTestStore(t))
	req := httptest.NewRequest(http.MethodGet, "/build.json", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %q", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("cache-control = %q, want no-store", got)
	}
	if got := rr.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("content-type = %q, want application/json", got)
	}
	var info struct {
		BuildID string `json:"buildId"`
		Schema  int    `json:"schema"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &info); err != nil {
		t.Fatalf("decode build.json: %v", err)
	}
	if info.BuildID != "v1.20.0-abc1234" || info.Schema != 7 {
		t.Errorf("build.json = %+v, want release identity and schema", info)
	}
}

func TestInjectHostedAppMetaIsIdempotentAndHandlesMinimalHTML(t *testing.T) {
	withHead := injectHostedAppMeta([]byte("<html><head><title>x</title></head></html>"))
	if strings.Count(string(withHead), hostedAppMeta) != 1 ||
		!strings.Contains(string(withHead), "<head>"+hostedAppMeta) {
		t.Fatalf("head injection = %q", withHead)
	}
	again := injectHostedAppMeta(withHead)
	if strings.Count(string(again), hostedAppMeta) != 1 {
		t.Fatalf("idempotent injection = %q", again)
	}
	minimal := injectHostedAppMeta([]byte("<title>x</title>"))
	if !strings.HasPrefix(string(minimal), hostedAppMeta) {
		t.Fatalf("minimal injection = %q", minimal)
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
