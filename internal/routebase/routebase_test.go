// SPDX-License-Identifier: MIT

package routebase

import "testing"

func TestNormalize(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"/", ""},
		{"   ", ""},
		{"/CashFlux/", "/CashFlux"},
		{"/CashFlux", "/CashFlux"},
		{"CashFlux/", "/CashFlux"},
		{"/a/b/", "/a/b"},
		{"https://monstercameron.github.io/CashFlux/", "/CashFlux"},
		{"http://localhost:8080/", ""},
		{"https://example.com", ""},
	}
	for _, c := range cases {
		if got := Normalize(c.in); got != c.want {
			t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestJoin(t *testing.T) {
	cases := []struct {
		base, path, want string
	}{
		// Empty base is a strict no-op (local dev / custom domain).
		{"", "/", "/"},
		{"", "/accounts", "/accounts"},
		{"", "*", "*"},
		{"", "/p/:slug", "/p/:slug"},
		// Sub-path base prefixes every concrete route.
		{"/CashFlux", "/", "/CashFlux/"},
		{"/CashFlux", "/accounts", "/CashFlux/accounts"},
		{"/CashFlux", "/p/:slug", "/CashFlux/p/:slug"},
		{"/CashFlux", "/p/taxes", "/CashFlux/p/taxes"},
		// The wildcard is never prefixed.
		{"/CashFlux", "*", "*"},
		// Defensive: a path missing its leading slash still joins cleanly.
		{"/CashFlux", "accounts", "/CashFlux/accounts"},
	}
	for _, c := range cases {
		if got := Join(c.base, c.path); got != c.want {
			t.Errorf("Join(%q, %q) = %q, want %q", c.base, c.path, got, c.want)
		}
	}
}

func TestStrip(t *testing.T) {
	cases := []struct {
		base, path, want string
	}{
		// Empty base: identity.
		{"", "/", "/"},
		{"", "/accounts", "/accounts"},
		// Sub-path base: strip back to the logical route.
		{"/CashFlux", "/CashFlux/accounts", "/accounts"},
		{"/CashFlux", "/CashFlux/", "/"},
		{"/CashFlux", "/CashFlux", "/"},
		{"/CashFlux", "/CashFlux/p/taxes", "/p/taxes"},
		// A path outside the base is left alone (defensive).
		{"/CashFlux", "/other", "/other"},
		// A look-alike prefix that isn't a path segment boundary is not stripped.
		{"/Cash", "/CashFlux/accounts", "/CashFlux/accounts"},
	}
	for _, c := range cases {
		if got := Strip(c.base, c.path); got != c.want {
			t.Errorf("Strip(%q, %q) = %q, want %q", c.base, c.path, got, c.want)
		}
	}
}

// TestStripJoinRoundTrip: Strip is the inverse of Join for concrete routes.
func TestStripJoinRoundTrip(t *testing.T) {
	for _, base := range []string{"", "/CashFlux", "/a/b"} {
		for _, p := range []string{"/", "/accounts", "/p/x"} {
			if got := Strip(base, Join(base, p)); got != p {
				t.Errorf("Strip(%q, Join(%q,%q)) = %q, want %q", base, base, p, got, p)
			}
		}
	}
}

// TestRoundTrip mirrors the real flow: a <base href> from index.html is
// normalized, then used to prefix routes; the local case must be identity.
func TestRoundTrip(t *testing.T) {
	pagesBase := Normalize("/CashFlux/")
	if pagesBase != "/CashFlux" {
		t.Fatalf("pages base = %q", pagesBase)
	}
	if got := Join(pagesBase, "/transactions"); got != "/CashFlux/transactions" {
		t.Errorf("pages join = %q", got)
	}
	localBase := Normalize("/")
	for _, p := range []string{"/", "/accounts", "/p/x", "*"} {
		if got := Join(localBase, p); got != p {
			t.Errorf("local Join(%q) = %q, want identity", p, got)
		}
	}
}
