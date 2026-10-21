// SPDX-License-Identifier: MIT

package app

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/atrest"
)

func TestENC33RefreshModeSelection(t *testing.T) {
	for _, tc := range []struct {
		name, endpoint, origin, mode string
		invalid                      bool
	}{
		{"hosted", "https://example.test", "https://example.test", "cookie", false},
		{"default port", "https://EXAMPLE.test:443/", "https://example.test", "cookie", false},
		{"same origin subpath", "https://example.test/budget/", "https://example.test", "cookie", false},
		{"cross origin", "https://server.test", "https://example.test", authRefreshModeEncrypted, false},
		{"different port", "https://example.test:8443", "https://example.test", authRefreshModeEncrypted, false},
		{"HTTP cannot install Secure cookie", "http://localhost:8080", "http://localhost:8080", authRefreshModeEncrypted, false},
		{"remote HTTP cannot carry credentials", "http://server.test", "http://page.test", "", true},
		{"no page origin", "https://server.test", "", authRefreshModeEncrypted, false},
		{"bad scheme", "file:///server", "https://example.test", "", true},
		{"userinfo", "https://secret@server.test", "https://example.test", "", true},
		{"query", "https://server.test?token=secret", "https://example.test", "", true},
		{"fragment", "https://server.test/#secret", "https://example.test", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := newRefreshScope("server-id", "account-id", tc.endpoint, tc.origin)
			if (err != nil) != tc.invalid || (!tc.invalid && s.Mode != tc.mode) {
				t.Fatalf("mode=%q err=%v, want %q invalid=%v", s.Mode, err, tc.mode, tc.invalid)
			}
		})
	}
	for _, ids := range [][2]string{{"", "account"}, {"server", ""}} {
		if _, err := newRefreshScope(ids[0], ids[1], "https://server.test", "https://page.test"); err == nil {
			t.Fatal("incomplete scope accepted")
		}
	}
}

func TestENC33CredentialScopeAndModeValidation(t *testing.T) {
	scope, err := newRefreshScope("server", "account", "https://server.test", "https://page.test")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := encodeRefreshCredential(scope, "private-refresh")
	if err != nil {
		t.Fatal(err)
	}
	c, err := decodeRefreshCredential(scope, raw)
	if err != nil || c.Token != "private-refresh" {
		t.Fatalf("round trip: %v", err)
	}
	for _, mutate := range []func(*refreshScope){
		func(s *refreshScope) { s.ServerID = "other-server" },
		func(s *refreshScope) { s.AccountID = "other-account" },
		func(s *refreshScope) { s.Endpoint = "https://other.test" },
		func(s *refreshScope) { s.Mode = "cookie" },
	} {
		other := scope
		mutate(&other)
		if _, err := decodeRefreshCredential(other, raw); err == nil {
			t.Fatal("foreign credential accepted")
		}
	}
	for _, broken := range []string{"{", "null", "{}", strings.Replace(raw, `"version":1`, `"version":2`, 1)} {
		if _, err := decodeRefreshCredential(scope, broken); err == nil {
			t.Fatal("invalid record accepted")
		}
	}
	cookie, _ := newRefreshScope("server", "account", "https://server.test", "https://server.test")
	if _, err := encodeRefreshCredential(cookie, "private-refresh"); err == nil {
		t.Fatal("cookie mode accepted a readable refresh token")
	}
	if _, err := encodeRefreshCredential(scope, ""); err == nil {
		t.Fatal("encrypted mode accepted no refresh token")
	}
	marker, err := encodeRefreshCredential(cookie, "")
	if err != nil || strings.Contains(marker, "refreshToken") {
		t.Fatalf("cookie marker: %s %v", marker, err)
	}
}

func TestENC33CredentialKeyIsUnambiguousAndCatalogueProtected(t *testing.T) {
	a := refreshScope{ServerID: "a:b", AccountID: "c"}
	b := refreshScope{ServerID: "a", AccountID: "b:c"}
	if a.key() == b.key() {
		t.Fatal("scope separators collided")
	}
	entry := atrest.ClassifyEntry(a.key())
	if entry.Key != "cred:refresh:" || entry.Class != atrest.MustEncrypt {
		t.Fatalf("credential catalogue: %+v", entry)
	}
	for _, key := range []string{"auth:access-token", "auth:refresh-token", "auth:csrf"} {
		if atrest.Classify(key) != atrest.ShouldNotPersist {
			t.Fatalf("generic credential %s became persistable", key)
		}
	}
}
