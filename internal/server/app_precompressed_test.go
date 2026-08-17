// SPDX-License-Identifier: MIT

package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// writeWasmFixture lays out an app dir with a raw wasm and both precompressed
// siblings, each with recognisably different bytes so a test can tell which one
// the handler chose.
func writeWasmFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"bin/main.wasm":    "RAW-WASM-BYTES",
		"bin/main.wasm.br": "BROTLI-BYTES",
		"bin/main.wasm.gz": "GZIP-BYTES",
		"index.html":       "<html><head></head><body></body></html>",
	} {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func getWasm(t *testing.T, dir, acceptEncoding string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/bin/main.wasm", nil)
	if acceptEncoding != "" {
		req.Header.Set("Accept-Encoding", acceptEncoding)
	}
	rr := httptest.NewRecorder()
	appHandler(Config{AppDir: dir}).ServeHTTP(rr, req)
	return rr
}

// The whole point: a browser that accepts brotli gets the smallest body, and the
// browser undoes the encoding itself.
func TestWasmPrefersBrotliWhenAccepted(t *testing.T) {
	rr := getWasm(t, writeWasmFixture(t), "gzip, deflate, br")
	if got := rr.Header().Get("Content-Encoding"); got != "br" {
		t.Errorf("Content-Encoding = %q, want br", got)
	}
	if got := rr.Body.String(); got != "BROTLI-BYTES" {
		t.Errorf("body = %q, want the brotli sibling", got)
	}
}

// instantiateStreaming REFUSES anything that is not application/wasm, so getting
// this wrong breaks the boot rather than merely slowing it. The type describes
// the body after the encoding is undone, not the file on disk.
func TestPrecompressedWasmKeepsTheWasmContentType(t *testing.T) {
	for _, enc := range []string{"br", "gzip"} {
		rr := getWasm(t, writeWasmFixture(t), enc)
		if got := rr.Header().Get("Content-Type"); got != "application/wasm" {
			t.Errorf("%s: Content-Type = %q, want application/wasm", enc, got)
		}
	}
}

// A shared cache must not hand a brotli body to a client that cannot read it.
func TestPrecompressedWasmVariesOnAcceptEncoding(t *testing.T) {
	rr := getWasm(t, writeWasmFixture(t), "br")
	if got := rr.Header().Get("Vary"); got == "" {
		t.Error("no Vary header on a negotiated response")
	}
}

func TestWasmFallsBackToGzipWhenBrotliIsNotAccepted(t *testing.T) {
	rr := getWasm(t, writeWasmFixture(t), "gzip")
	if got := rr.Header().Get("Content-Encoding"); got != "gzip" {
		t.Errorf("Content-Encoding = %q, want gzip", got)
	}
	if got := rr.Body.String(); got != "GZIP-BYTES" {
		t.Errorf("body = %q, want the gzip sibling", got)
	}
}

// A client that accepts nothing gets the real file. This is also the dev-server
// case, and it must keep working or local boot breaks.
func TestWasmServesRawWhenNoEncodingIsAccepted(t *testing.T) {
	rr := getWasm(t, writeWasmFixture(t), "")
	if got := rr.Header().Get("Content-Encoding"); got != "" {
		t.Errorf("Content-Encoding = %q, want none", got)
	}
	if got := rr.Body.String(); got != "RAW-WASM-BYTES" {
		t.Errorf("body = %q, want the raw file", got)
	}
}

// No siblings on disk is the ordinary local build. It must serve the raw binary
// rather than 404 or serve nothing.
func TestWasmWithoutSiblingsStillServesRaw(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin", "main.wasm"), []byte("RAW"), 0o600); err != nil {
		t.Fatal(err)
	}
	rr := getWasm(t, dir, "br, gzip")
	if rr.Code != http.StatusOK || rr.Body.String() != "RAW" {
		t.Errorf("code=%d body=%q, want 200/RAW", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Encoding"); got != "" {
		t.Errorf("Content-Encoding = %q on a file with no siblings", got)
	}
}

// q=0 is how a client says "not this one".
func TestWasmHonoursAnExplicitEncodingRefusal(t *testing.T) {
	rr := getWasm(t, writeWasmFixture(t), "br;q=0, gzip")
	if got := rr.Header().Get("Content-Encoding"); got != "gzip" {
		t.Errorf("Content-Encoding = %q, want gzip — br was refused with q=0", got)
	}
}

// Only .wasm is negotiated; everything else is small enough that the extra stats
// would cost more than they save.
func TestNonWasmAssetsAreNotNegotiated(t *testing.T) {
	dir := writeWasmFixture(t)
	if err := os.WriteFile(filepath.Join(dir, "app.js"), []byte("JS"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.js.br"), []byte("JS-BR"), 0o600); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	req.Header.Set("Accept-Encoding", "br")
	rr := httptest.NewRecorder()
	appHandler(Config{AppDir: dir}).ServeHTTP(rr, req)
	if got := rr.Body.String(); got != "JS" {
		t.Errorf("body = %q, want the plain file", got)
	}
}
