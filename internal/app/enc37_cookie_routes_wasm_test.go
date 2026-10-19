// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strings"
	"syscall/js"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/prefs"
)

func enc37FakeFetch(t *testing.T, status int, body string) {
	t.Helper()
	old := js.Global().Get("fetch")
	js.Global().Set("fetch", js.Global().Get("Function").New("status", "body", `
return function(url, opts) {
  globalThis.__enc37Request = {url, method:opts.method, credentials:opts.credentials,
    authorization:opts.headers.get("Authorization"), csrf:opts.headers.get("X-CashFlux-CSRF")};
  return Promise.resolve({status, text(){return Promise.resolve(body)}});
};`).Invoke(status, body))
	t.Cleanup(func() {
		js.Global().Set("fetch", old)
		js.Global().Set("__enc37Request", js.Undefined())
	})
}

func enc37Wait(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("fake fetch callback did not complete")
		return nil
	}
}

func TestENC37CookieLogoutUsesRefreshRouteAndSurfacesFailure(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		wantErr bool
	}{
		{name: "success", status: 204},
		{name: "server failure", status: 503, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			enc37FakeFetch(t, tc.status, "")
			done := make(chan error, 1)
			signOutBackendOAuth("https://app.test", "enc37-access", "enc37-csrf", func(err error) { done <- err })
			err := enc37Wait(t, done)
			request := js.Global().Get("__enc37Request")
			if got := request.Get("url").String(); got != "https://app.test/v1/auth/refresh/logout" {
				t.Fatalf("logout URL = %q", got)
			}
			if request.Get("method").String() != "POST" || request.Get("credentials").String() != "include" {
				t.Fatal("cookie logout omitted POST or credentials: include")
			}
			if request.Get("authorization").String() != "Bearer enc37-access" || request.Get("csrf").String() != "enc37-csrf" {
				t.Fatal("cookie logout omitted access bearer or double-submit CSRF header")
			}
			if (err != nil) != tc.wantErr {
				t.Fatalf("logout error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr && !strings.Contains(err.Error(), "HTTP 503") {
				t.Fatalf("failure not surfaced: %v", err)
			}
		})
	}
}

func TestENC37DeviceRoutesIncludeCookieBearerAndDeleteCSRF(t *testing.T) {
	oldLocation, oldDocument := js.Global().Get("location"), js.Global().Get("document")
	js.Global().Set("location", map[string]any{"origin": "https://app.test"})
	js.Global().Set("document", map[string]any{"cookie": "cashflux_csrf=enc37-csrf"})
	t.Cleanup(func() {
		js.Global().Set("location", oldLocation)
		js.Global().Set("document", oldDocument)
	})

	t.Run("list", func(t *testing.T) {
		enc37FakeFetch(t, 200, `{"sessions":[{"familyId":"family-current","current":true}]}`)
		done := make(chan []sessionFamily, 1)
		failed := make(chan error, 1)
		fetchSessions("https://app.test", "enc37-access", func(list []sessionFamily) { done <- list }, func(err error) { failed <- err })
		select {
		case list := <-done:
			if len(list) != 1 || list[0].FamilyID != "family-current" || !list[0].Current {
				t.Fatalf("sessions = %#v", list)
			}
		case err := <-failed:
			t.Fatal(err)
		case <-time.After(5 * time.Second):
			t.Fatal("device list callback did not complete")
		}
		request := js.Global().Get("__enc37Request")
		if request.Get("url").String() != "https://app.test/v1/auth/refresh/sessions" || request.Get("method").String() != "GET" || request.Get("credentials").String() != "include" {
			t.Fatal("device list did not use cookie-enabled refresh sessions route")
		}
		if request.Get("authorization").String() != "Bearer enc37-access" {
			t.Fatal("device list omitted access bearer")
		}
	})

	t.Run("revoke", func(t *testing.T) {
		enc37FakeFetch(t, 204, "")
		done := make(chan error, 1)
		revokeSession("https://app.test", "enc37-access", cookieSessionCSRF(prefs.Prefs{ServerURL: "https://app.test"}), "family-123", func(err error) { done <- err })
		if err := enc37Wait(t, done); err != nil {
			t.Fatal(err)
		}
		request := js.Global().Get("__enc37Request")
		if request.Get("url").String() != "https://app.test/v1/auth/refresh/sessions/family-123" || request.Get("method").String() != "DELETE" || request.Get("credentials").String() != "include" {
			t.Fatal("revoke did not use cookie-enabled refresh sessions route")
		}
		if request.Get("authorization").String() != "Bearer enc37-access" || request.Get("csrf").String() != "enc37-csrf" {
			t.Fatal("revoke omitted access bearer or double-submit CSRF header")
		}
	})

	t.Run("failure surfaced", func(t *testing.T) {
		enc37FakeFetch(t, 403, "")
		done := make(chan error, 1)
		revokeSession("https://app.test", "enc37-access", "enc37-csrf", "family-123", func(err error) { done <- err })
		if err := enc37Wait(t, done); err == nil || !strings.Contains(err.Error(), "HTTP 403") {
			t.Fatalf("revoke failure = %v", err)
		}
	})
}
