// SPDX-License-Identifier: MIT

package routebase

import (
	"strings"
	"testing"
)

// realRoutes mirrors the screen registry (internal/screens.All()) plus the
// patterns app.Run registers. Kept here so the sub-path contract is exercised
// against the actual route shape the app uses.
var realRoutes = []string{
	"/", "/accounts", "/transactions", "/budgets", "/goals", "/todo",
	"/planning", "/allocate", "/reports", "/subscriptions", "/bills", "/split",
	"/insights", "/documents", "/customize", "/artifacts", "/workflows",
	"/members", "/categories", "/rules",
}

// TestSubPathRoutingContract is the deep guard for serving the SPA under a base
// prefix (GitHub Pages /CashFlux/, /app/v2, etc.). It mirrors the full B30 cycle
// the router relies on — register at Join(base,route), match the live pathname,
// and Strip it back to the logical route for the active-highlight comparison —
// for several bases, and asserts every invariant the routing depends on.
func TestSubPathRoutingContract(t *testing.T) {
	bases := []string{"", "/CashFlux", "/app", "/a/b", "/Repo-Name"}
	for _, base := range bases {
		reg := map[string]string{} // logical route -> registered path
		seenReg := map[string]bool{}
		for _, route := range realRoutes {
			r := Join(base, route)
			reg[route] = r

			// 1. Registration paths stay unique under the prefix (a collision would
			//    silently drop a route to the "*" catch-all = "not navigable").
			if seenReg[r] {
				t.Errorf("base %q: route %q collides with another at %q", base, route, r)
			}
			seenReg[r] = true

			// 2. Non-empty base actually prefixes the route.
			if base != "" {
				if route == "/" {
					if r != base+"/" {
						t.Errorf("base %q: home registered at %q, want %q", base, r, base+"/")
					}
				} else if !strings.HasPrefix(r, base+"/") {
					t.Errorf("base %q: route %q registered at %q, missing prefix", base, route, r)
				}
			} else if r != route {
				t.Errorf("empty base must be a no-op: route %q -> %q", route, r)
			}

			// 3. When the browser is AT the registered path, Strip recovers the
			//    logical route — this is exactly what the rail active-highlight and
			//    breadcrumb compare against, so it must hold for every base.
			if got := Strip(base, r); got != route {
				t.Errorf("base %q: Strip(%q) = %q, want logical %q", base, r, got, route)
			}
		}

		// 4. The wildcard catch-all is never prefixed (it must match any unmatched
		//    path regardless of base).
		if got := Join(base, "*"); got != "*" {
			t.Errorf("base %q: wildcard prefixed to %q", base, got)
		}

		// 5. The /p/:slug custom-page pattern and a concrete slug both prefix and
		//    strip cleanly.
		if got := Strip(base, Join(base, "/p/taxes")); got != "/p/taxes" {
			t.Errorf("base %q: custom page slug round-trip = %q", base, got)
		}
		if base != "" && !strings.HasPrefix(Join(base, "/p/:slug"), base+"/p/") {
			t.Errorf("base %q: /p/:slug pattern not prefixed", base)
		}

		// 6. DefaultRoute resolves to the base root.
		want := "/"
		if base != "" {
			want = base + "/"
		}
		if got := Join(base, "/"); got != want {
			t.Errorf("base %q: default route = %q, want %q", base, got, want)
		}

		// 7. A live pathname under the base (as window.location.pathname would read)
		//    strips back to the logical route for the active comparison.
		for _, route := range realRoutes {
			pathname := reg[route]
			if got := Strip(base, pathname); got != route {
				t.Errorf("base %q: live pathname %q strips to %q, want %q", base, pathname, got, route)
			}
		}
	}
}

// TestNormalizeFromLiveBaseHref covers the values index.html actually writes into
// <base href> across deployments, including the resolved absolute form a browser
// reports for base.href.
func TestNormalizeFromLiveBaseHref(t *testing.T) {
	cases := []struct{ href, want string }{
		{"/", ""},                   // local dev / custom domain root
		{"/CashFlux/", "/CashFlux"}, // raw getAttribute on Pages
		{"https://monstercameron.github.io/CashFlux/", "/CashFlux"}, // resolved base.href on Pages
		{"https://example.com/", ""},                                // custom domain, absolute
		{"https://example.com", ""},                                 // no trailing slash
		{"/deep/sub/path/", "/deep/sub/path"},                       // multi-segment base
		{"  /CashFlux/  ", "/CashFlux"},                             // whitespace tolerant
	}
	for _, c := range cases {
		got := Normalize(c.href)
		if got != c.want {
			t.Errorf("Normalize(%q) = %q, want %q", c.href, got, c.want)
		}
		// Idempotent: normalizing an already-normalized prefix is a no-op.
		if again := Normalize(got); again != got {
			t.Errorf("Normalize not idempotent: Normalize(%q) = %q", got, again)
		}
	}
}

// TestStripLeavesForeignPaths makes sure Strip never mangles a path that isn't
// under the base (defensive: a stray absolute link shouldn't be silently rewritten).
func TestStripLeavesForeignPaths(t *testing.T) {
	cases := []struct{ base, path string }{
		{"/CashFlux", "/elsewhere"},
		{"/CashFlux", "/CashFluxXtra/accounts"}, // look-alike, not a segment boundary
		{"/app", "/application"},
	}
	for _, c := range cases {
		if got := Strip(c.base, c.path); got != c.path {
			t.Errorf("Strip(%q, %q) = %q, want it unchanged", c.base, c.path, got)
		}
	}
}
