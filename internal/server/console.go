// SPDX-License-Identifier: MIT

//go:build !js || !wasm

package server

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// consoleHandler returns an http.Handler that serves the operator console
// SPA from cfg.ConsoleDir under the /console/ prefix.
// Unknown sub-paths (non-asset requests) fall back to index.html for SPA routing.
func consoleHandler(cfg Config) http.Handler {
	return spaStaticHandler("/console/", cfg.ConsoleDir)
}

// spaStaticHandler serves files from dir while keeping every request inside its
// fs.FS root. Missing extensionless paths use index.html for client-side routes.
func spaStaticHandler(prefix, dir string) http.Handler {
	root, err := fs.Sub(os.DirFS(dir), ".")
	if err != nil {
		return http.NotFoundHandler()
	}
	files := http.FileServer(http.FS(root))
	return http.StripPrefix(prefix, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, "/")
		if !safeStaticPath(r.URL, rel) {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}
		clean := path.Clean(rel)
		if clean == "." {
			clean = "index.html"
		}
		if _, err := fs.Stat(root, clean); err == nil {
			files.ServeHTTP(w, r)
			return
		} else if !errors.Is(err, fs.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		if path.Ext(clean) != "" {
			http.NotFound(w, r)
			return
		}
		fallback := r.Clone(r.Context())
		fallback.URL.Path = "/"
		files.ServeHTTP(w, fallback)
	}))
}

func safeStaticPath(u *url.URL, rel string) bool {
	if u == nil || strings.ContainsAny(rel, "\\") || strings.HasPrefix(rel, "/") || filepath.IsAbs(rel) || filepath.VolumeName(rel) != "" {
		return false
	}
	for _, segment := range strings.Split(rel, "/") {
		if segment == ".." {
			return false
		}
	}
	// URL.Path is decoded by net/http. Also reject a raw encoded separator so
	// alternate parsing by a proxy cannot turn a checked name into traversal.
	if strings.Contains(strings.ToLower(u.RawPath), "%5c") {
		return false
	}
	return true
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
