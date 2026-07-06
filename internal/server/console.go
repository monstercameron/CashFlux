// SPDX-License-Identifier: MIT

package server

import (
	"encoding/json"
	"net"
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
		// Clean the request path as if rooted at "/" BEFORE joining, so a crafted
		// "../.." can't escape ConsoleDir (path traversal). filepath.Clean("/"+rel)
		// collapses any leading "../" against the root, keeping the join contained.
		rel := filepath.Clean("/" + filepath.FromSlash(strings.TrimPrefix(r.URL.Path, "/")))
		path := filepath.Join(dir, rel)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		fs.ServeHTTP(w, r)
	}))
}

// devCredsResponse is the JSON body returned by GET /console/devcreds.
type devCredsResponse struct {
	AdminToken string `json:"adminToken"`
}

// devCredsHandler handles GET /console/devcreds.
//
// Security gate: this endpoint is available ONLY when ALL three conditions hold:
//  1. cfg.DevMode is true (set via CASHFLUX_SERVER_DEV_MODE=true) — disabled by default;
//  2. the request originates from a loopback address (127.0.0.1 / ::1 / localhost);
//  3. cfg.Token is non-empty (the raw admin token is available in config).
//
// If any condition is not met the handler responds with 404 (Not Found) so
// that production deployments expose no information about the endpoint's existence.
// DevMode must never be set to true in production.
func devCredsHandler(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Gate 1: dev mode must be explicitly enabled.
		if !cfg.DevMode {
			http.NotFound(w, r)
			return
		}
		// Gate 2: request must come from a loopback address.
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil || !isLoopbackHost(host) {
			http.NotFound(w, r)
			return
		}
		// Gate 3: a raw token must actually be configured.
		if cfg.Token == "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(devCredsResponse{AdminToken: cfg.Token}); err != nil {
			http.Error(w, "encode failed", http.StatusInternalServerError)
		}
	}
}
