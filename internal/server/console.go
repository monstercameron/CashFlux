// SPDX-License-Identifier: MIT

package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// consoleHandler returns an http.Handler that serves the operator console
// SPA from cfg.ConsoleDir under the /console/ prefix.
// Unknown sub-paths (non-asset requests) fall back to index.html for SPA routing.
func consoleHandler(cfg Config) http.Handler {
	dir := cfg.ConsoleDir
	fs := http.FileServer(http.Dir(dir))
	return http.StripPrefix("/console/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve the file if it exists; fall back to index.html for SPA routes.
		path := filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(r.URL.Path, "/")))
		if _, err := os.Stat(path); os.IsNotExist(err) {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		fs.ServeHTTP(w, r)
	}))
}
