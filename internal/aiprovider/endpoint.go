// SPDX-License-Identifier: MIT

package aiprovider

import "strings"

// ResolveBaseURL picks the effective API base URL for a call (AG18): a non-blank,
// user-supplied override wins so the app can point at any OpenAI-compatible
// endpoint — a local model server (Ollama, LM Studio), a proxy, or an alternate
// provider — and falls back to the built-in default otherwise. The override is
// trimmed and its trailing slashes removed so callers can safely append "/models"
// or "/chat/completions" without doubling the separator. A blank override or a
// blank fallback each degrade gracefully rather than producing a malformed URL.
func ResolveBaseURL(override, fallback string) string {
	if o := strings.TrimRight(strings.TrimSpace(override), "/"); o != "" {
		return o
	}
	return strings.TrimRight(strings.TrimSpace(fallback), "/")
}

// IsLocalEndpoint reports whether a base URL points at a loopback/local host — the
// honest "no key leaves the house" path. Used to soften the UI (a local endpoint
// needs no real API key) and to label the active endpoint. It inspects the host
// only, so a path or query never fools the check.
func IsLocalEndpoint(baseURL string) bool {
	s := strings.TrimSpace(strings.ToLower(baseURL))
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	host := s
	// Bracketed IPv6 literal, e.g. [::1]:8080 -> ::1.
	if strings.HasPrefix(host, "[") {
		if j := strings.Index(host, "]"); j >= 0 {
			host = host[1:j]
		}
	} else if i := strings.LastIndex(host, ":"); i >= 0 {
		host = host[:i]
	}
	switch host {
	case "localhost", "127.0.0.1", "::1", "0.0.0.0", "host.docker.internal":
		return true
	}
	return strings.HasSuffix(host, ".local")
}
