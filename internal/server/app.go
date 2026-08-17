// SPDX-License-Identifier: MIT

package server

import (
	"bytes"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"
)

// appHandler serves the primary CashFlux SPA from cfg.AppDir. Existing files
// are served directly and extensionless browser routes fall back to index.html.
// os.DirFS + fs.ValidPath keep all lookups rooted beneath AppDir.
func appHandler(cfg Config) http.Handler {
	root := os.DirFS(cfg.AppDir)
	files := http.FileServerFS(root)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if appReservedPath(r.URL.Path) {
			http.NotFound(w, r)
			return
		}

		rel := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if rel == "" {
			rel = "."
		}
		if rel != "." && !fs.ValidPath(rel) {
			http.NotFound(w, r)
			return
		}

		if info, err := fs.Stat(root, rel); err == nil {
			headerName := rel
			if info.IsDir() {
				index := path.Join(rel, "index.html")
				if _, err := fs.Stat(root, index); err != nil {
					http.NotFound(w, r)
					return
				}
				headerName = index
			}
			setAppAssetHeaders(w, headerName)
			if path.Base(headerName) == "index.html" {
				serveHostedAppIndex(w, r, root, headerName)
				return
			}
			if servePrecompressed(w, r, root, rel) {
				return
			}
			files.ServeHTTP(w, r)
			return
		}

		// A missing asset must stay a 404. Only a browser navigation (HTML
		// accepted) or an extensionless application route gets the SPA shell.
		if path.Ext(rel) != "" {
			http.NotFound(w, r)
			return
		}
		if _, err := fs.Stat(root, "index.html"); err != nil {
			http.NotFound(w, r)
			return
		}
		setAppAssetHeaders(w, "index.html")
		serveHostedAppIndex(w, r, root, "index.html")
	})
}

const hostedAppMeta = `<meta name="cashflux-hosted-app" content="true" />`

// serveHostedAppIndex stamps only server-owned HTML with a marker the WASM can
// inspect before rendering a route. The source web/index.html stays portable,
// so GitHub Pages, local dev, and offline bundles remain normal ungated apps.
func serveHostedAppIndex(w http.ResponseWriter, r *http.Request, root fs.FS, name string) {
	file, err := root.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	body, err := fs.ReadFile(root, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	body = injectHostedAppMeta(body)
	http.ServeContent(w, r, name, info.ModTime(), bytes.NewReader(body))
}

func injectHostedAppMeta(body []byte) []byte {
	if bytes.Contains(body, []byte(hostedAppMeta)) {
		return body
	}
	for _, head := range [][]byte{[]byte("<head>"), []byte("<HEAD>")} {
		if bytes.Contains(body, head) {
			replacement := append(append([]byte(nil), head...), []byte(hostedAppMeta)...)
			return bytes.Replace(body, head, replacement, 1)
		}
	}
	return append([]byte(hostedAppMeta), body...)
}

func acceptsHTML(r *http.Request) bool {
	return strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/html")
}

func setAppAssetHeaders(w http.ResponseWriter, rel string) {
	base := path.Base(rel)
	ext := strings.ToLower(path.Ext(base))
	w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")

	switch {
	case rel == "." || base == "index.html" || ext == ".html":
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case base == "sw.js" || base == "services-worker.js" || base == "wasm_exec.js" ||
		ext == ".wasm" || strings.HasSuffix(base, ".wasm.gz"):
		// These files form one deploy unit. Revalidation prevents a worker or
		// runtime glue file from loading a mismatched WASM protocol/version.
		w.Header().Set("Cache-Control", "no-cache")
		if ext == ".wasm" {
			w.Header().Set("Content-Type", "application/wasm")
		}
	case ext == ".webmanifest":
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Type", "application/manifest+json")
	default:
		w.Header().Set("Cache-Control", "public, max-age=3600")
		if contentType := mime.TypeByExtension(ext); contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
	}
}

// appReservedPath prevents an unknown server/control-plane URL from becoming
// an apparently successful SPA response through the root catch-all.
func appReservedPath(requestPath string) bool {
	for _, prefix := range []string{
		"/v1", "/grpc", "/console", "/portal", "/legal",
		"/livez", "/healthz", "/readyz", "/status", "/metrics",
	} {
		if requestPath == prefix || strings.HasPrefix(requestPath, prefix+"/") {
			return true
		}
	}
	return false
}

// servePrecompressed serves a .br or .gz sibling of rel when the client accepts
// that encoding, and reports whether it handled the request.
//
// # Why this exists
//
// The app cannot paint until its WebAssembly has arrived, and that binary is the
// single largest thing this server ships. Measured against the droplet: 17.4 MB
// gzipped takes between 12 seconds and two minutes depending on how much outbound
// bandwidth is available at that moment, and the whole time the user is looking at
// a splash screen. Brotli is 11.4 MB of the same binary — a third less — and the
// browser decompresses Content-Encoding: br natively, so nothing has to be
// unpacked in JavaScript.
//
// Only .wasm is negotiated. Everything else the app serves is small enough that
// the extra stat calls per request would cost more than the bytes they save.
//
// The Content-Type stays the UNCOMPRESSED type, because that is what it is once
// the encoding is undone — and WebAssembly.instantiateStreaming refuses anything
// that is not application/wasm, so getting this wrong breaks the boot rather than
// merely slowing it.
func servePrecompressed(w http.ResponseWriter, r *http.Request, root fs.FS, rel string) bool {
	if path.Ext(rel) != ".wasm" {
		return false
	}
	accept := r.Header.Get("Accept-Encoding")
	// Brotli first: it is the smaller of the two, and a client that accepts br
	// accepts gzip as well, so asking in the other order would never choose it.
	for _, c := range []struct{ enc, ext string }{{"br", ".br"}, {"gzip", ".gz"}} {
		if !acceptsEncoding(accept, c.enc) {
			continue
		}
		name := rel + c.ext
		f, err := root.Open(name)
		if err != nil {
			continue
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil || info.IsDir() {
			continue
		}
		rs, ok := f.(io.ReadSeeker)
		if !ok {
			// http.ServeContent needs to seek for range requests. Without it, fall
			// through to the plain file server rather than serving something a range
			// request would mangle.
			continue
		}
		w.Header().Set("Content-Encoding", c.enc)
		// The cached copy depends on which encoding was negotiated, so a shared
		// cache must not hand a br body to a client that cannot read it.
		w.Header().Add("Vary", "Accept-Encoding")
		w.Header().Set("Content-Type", "application/wasm")
		http.ServeContent(w, r, path.Base(rel), info.ModTime(), rs)
		return true
	}
	return false
}

// acceptsEncoding reports whether an Accept-Encoding header offers enc. It is a
// deliberately small test: it looks for the token and rejects an explicit q=0,
// which is the only way a client says "not this one" in practice.
func acceptsEncoding(header, enc string) bool {
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, params, _ := strings.Cut(part, ";")
		if !strings.EqualFold(strings.TrimSpace(name), enc) {
			continue
		}
		if strings.Contains(strings.ReplaceAll(params, " ", ""), "q=0") &&
			!strings.Contains(strings.ReplaceAll(params, " ", ""), "q=0.") {
			return false
		}
		return true
	}
	return false
}
