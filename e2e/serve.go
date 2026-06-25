// Command serve is a tiny static file server for E2E tests: it serves the built
// web/ app directory with a SPA history fallback (extensionless paths -> index.html)
// and the correct application/wasm MIME, since `gwc dev` can't reliably serve the
// HTML shell (see B1). Native Go (no build tags); run with `go run e2e/serve.go`.
//
// Wasm compression (C314): for .wasm requests, negotiates Accept-Encoding and serves
// a precompressed sibling (<file>.br or <file>.gz) if present, with the appropriate
// Content-Encoding header. Compression is never done on the fly — only precompressed
// siblings produced at build time (e.g. by the CI workflow) are used.
package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// serveWasm handles a request for a .wasm file with transparent precompressed-sibling
// negotiation. It sets Content-Type: application/wasm and Vary: Accept-Encoding on
// every response. If the client signals brotli support and a .br sibling exists on
// disk it is served; otherwise gzip with a .gz sibling; otherwise the raw .wasm.
func serveWasm(w http.ResponseWriter, r *http.Request, full string) {
	accept := r.Header.Get("Accept-Encoding")
	w.Header().Set("Content-Type", "application/wasm")
	w.Header().Set("Vary", "Accept-Encoding")

	if strings.Contains(accept, "br") {
		if _, err := os.Stat(full + ".br"); err == nil {
			w.Header().Set("Content-Encoding", "br")
			http.ServeFile(w, r, full+".br")
			return
		}
	}
	if strings.Contains(accept, "gzip") {
		if _, err := os.Stat(full + ".gz"); err == nil {
			w.Header().Set("Content-Encoding", "gzip")
			http.ServeFile(w, r, full+".gz")
			return
		}
	}
	http.ServeFile(w, r, full)
}

func main() {
	root, port := "web", "8099"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	if len(os.Args) > 2 {
		port = os.Args[2]
	}
	handler := func(w http.ResponseWriter, r *http.Request) {
		clean := filepath.Clean(r.URL.Path)
		full := filepath.Join(root, clean)
		if strings.HasSuffix(clean, ".wasm") {
			if info, err := os.Stat(full); err == nil && !info.IsDir() {
				serveWasm(w, r, full)
				return
			}
			http.NotFound(w, r)
			return
		}
		if strings.HasSuffix(clean, ".webmanifest") {
			w.Header().Set("Content-Type", "application/manifest+json")
		}
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			http.ServeFile(w, r, full)
			return
		}
		// SPA history fallback: a clean route (no file extension) boots the shell.
		if !strings.Contains(filepath.Base(clean), ".") {
			http.ServeFile(w, r, filepath.Join(root, "index.html"))
			return
		}
		http.NotFound(w, r)
	}
	log.Printf("e2e: serving %s at http://127.0.0.1:%s", root, port)
	log.Fatal(http.ListenAndServe("127.0.0.1:"+port, http.HandlerFunc(handler)))
}
