// Command serve is a tiny static file server for E2E tests: it serves the built
// web/ app directory with a SPA history fallback (extensionless paths -> index.html)
// and the correct application/wasm MIME, since `gwc dev` can't reliably serve the
// HTML shell (see B1). Native Go (no build tags); run with `go run e2e/serve.go`.
package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

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
			w.Header().Set("Content-Type", "application/wasm")
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
