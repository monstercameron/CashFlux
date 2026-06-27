// Tests for the wasm-compression handler in serve.go (C314).
// Verifies: (a) gzip Accept-Encoding returns Content-Encoding: gzip + decompressible body;
// (b) no Accept-Encoding returns identity bytes + correct Content-Type;
// (c) Vary: Accept-Encoding is present on every wasm response.
package main

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// makeWasmFixture writes a small synthetic wasm body to a temp dir and returns
// (dir, fullPath) so tests don't need the real 60 MB binary.
func makeWasmFixture(t *testing.T, body []byte) (dir, full string) {
	t.Helper()
	dir = t.TempDir()
	full = filepath.Join(dir, "main.wasm")
	if err := os.WriteFile(full, body, 0o644); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return dir, full
}

// TestServeWasm_GzipOnTheFly: client accepts gzip, no .gz sibling → on-the-fly compress.
func TestServeWasm_GzipOnTheFly(t *testing.T) {
	original := []byte("fake wasm bytes for gzip on-the-fly test")
	_, full := makeWasmFixture(t, original)

	req := httptest.NewRequest(http.MethodGet, "/bin/main.wasm", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	serveWasm(rec, req, full)

	resp := rec.Result()
	if got := resp.Header.Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding: got %q, want %q", got, "gzip")
	}
	if got := resp.Header.Get("Content-Type"); got != "application/wasm" {
		t.Fatalf("Content-Type: got %q, want %q", got, "application/wasm")
	}
	if got := resp.Header.Get("Vary"); got != "Accept-Encoding" {
		t.Fatalf("Vary: got %q, want %q", got, "Accept-Encoding")
	}

	gr, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer gr.Close()
	got, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	if string(got) != string(original) {
		t.Fatalf("decompressed body mismatch: got %q, want %q", got, original)
	}
}

// TestServeWasm_Identity: no Accept-Encoding → raw bytes + correct headers.
func TestServeWasm_Identity(t *testing.T) {
	original := []byte("fake wasm bytes for identity test")
	_, full := makeWasmFixture(t, original)

	req := httptest.NewRequest(http.MethodGet, "/bin/main.wasm", nil)
	// No Accept-Encoding header.
	rec := httptest.NewRecorder()

	serveWasm(rec, req, full)

	resp := rec.Result()
	if got := resp.Header.Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding: got %q, want empty (identity)", got)
	}
	if got := resp.Header.Get("Content-Type"); got != "application/wasm" {
		t.Fatalf("Content-Type: got %q, want %q", got, "application/wasm")
	}
	if got := resp.Header.Get("Vary"); got != "Accept-Encoding" {
		t.Fatalf("Vary: got %q, want %q", got, "Accept-Encoding")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != string(original) {
		t.Fatalf("identity body mismatch: got %q, want %q", body, original)
	}
}

// TestServeWasm_VaryAlwaysPresent: Vary header is set regardless of encoding path.
func TestServeWasm_VaryAlwaysPresent(t *testing.T) {
	original := []byte("fake wasm bytes for vary header test")
	_, full := makeWasmFixture(t, original)

	for _, ae := range []string{"", "gzip", "br", "gzip, br"} {
		req := httptest.NewRequest(http.MethodGet, "/bin/main.wasm", nil)
		if ae != "" {
			req.Header.Set("Accept-Encoding", ae)
		}
		rec := httptest.NewRecorder()
		serveWasm(rec, req, full)
		resp := rec.Result()
		if got := resp.Header.Get("Vary"); got != "Accept-Encoding" {
			t.Fatalf("Accept-Encoding=%q: Vary got %q, want %q", ae, got, "Accept-Encoding")
		}
	}
}

// TestServeWasm_PrecompressedGzipSibling: client accepts gzip and a .gz sibling exists →
// serve the sibling, not on-the-fly.
func TestServeWasm_PrecompressedGzipSibling(t *testing.T) {
	original := []byte("fake wasm bytes for sibling test")
	_, full := makeWasmFixture(t, original)

	// Write a fake .gz sibling (it doesn't need to be valid gzip for the header test).
	sibling := []byte("PRECOMPRESSED")
	if err := os.WriteFile(full+".gz", sibling, 0o644); err != nil {
		t.Fatalf("write sibling: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/bin/main.wasm", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	serveWasm(rec, req, full)

	resp := rec.Result()
	if got := resp.Header.Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding: got %q, want %q", got, "gzip")
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != string(sibling) {
		t.Fatalf("expected sibling bytes %q, got %q", sibling, body)
	}
}
