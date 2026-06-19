// Package routebase is the pure core for serving the SPA under a URL sub-path
// (e.g. GitHub Pages project sites at https://user.github.io/CashFlux/). The
// history router matches and pushes raw pathnames, so when the app lives under a
// base prefix every registered route and every Navigate target must carry that
// prefix; otherwise matching fails and navigation strips the base back to the
// origin root (B30).
//
// This package holds only the string logic — deriving the prefix from the
// document's <base href> and joining it onto a route path — so it is pure Go,
// no syscall/js, and unit-tested on native Go. The wasm layer reads the live
// <base href> and feeds it here.
package routebase

import "strings"

// Normalize turns a <base href> value into a route prefix: the path portion with
// any trailing slash removed. The server root ("/" or "") yields "" (no prefix),
// so the common local-dev / custom-domain case is a clean no-op. A sub-path base
// like "/CashFlux/" yields "/CashFlux". An absolute href ("https://host/CashFlux/")
// is accepted too — only its path is kept.
func Normalize(baseHref string) string {
	href := strings.TrimSpace(baseHref)
	if href == "" {
		return ""
	}
	// Drop a scheme + host if an absolute URL was passed, keeping the path.
	if i := strings.Index(href, "://"); i >= 0 {
		rest := href[i+3:]
		if slash := strings.IndexByte(rest, '/'); slash >= 0 {
			href = rest[slash:]
		} else {
			href = "/"
		}
	}
	href = strings.TrimRight(href, "/")
	if href == "" {
		return ""
	}
	if !strings.HasPrefix(href, "/") {
		href = "/" + href
	}
	return href
}

// Strip removes the base prefix from a raw pathname, returning the logical route
// path so comparisons against unprefixed route literals work under a sub-path
// deployment. An empty base returns the path unchanged; a path that doesn't sit
// under the base is returned as-is. The base itself ("/CashFlux") or its root
// ("/CashFlux/") maps back to "/".
func Strip(base, path string) string {
	if base == "" {
		return path
	}
	if path == base {
		return "/"
	}
	if strings.HasPrefix(path, base+"/") {
		return path[len(base):]
	}
	return path
}

// Join prefixes a route path with the base. An empty base returns the path
// unchanged (the local-dev no-op). The wildcard "*" — the router's not-found
// registration — is never prefixed. Paths are expected to start with "/"; the
// root "/" under a base becomes "<base>/" (e.g. "/CashFlux/").
func Join(base, path string) string {
	if base == "" || path == "*" {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}
