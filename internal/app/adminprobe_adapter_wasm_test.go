// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/adminclient"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/screens"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

type adminAdapterCall struct {
	Path          string
	Query         string
	Authorization string
}

type adminAdapterDoer struct {
	mu             sync.Mutex
	calls          []adminAdapterCall
	overviewStatus int
	usersStatus    int
}

func (d *adminAdapterDoer) Do(req *http.Request) (*http.Response, error) {
	d.mu.Lock()
	d.calls = append(d.calls, adminAdapterCall{
		Path: req.URL.Path, Query: req.URL.RawQuery,
		Authorization: req.Header.Get("Authorization"),
	})
	d.mu.Unlock()

	status := d.overviewStatus
	body := `{"totalUsers":7,"day":"2026-10-21"}`
	if req.URL.Path == "/v1/admin/users" {
		status = d.usersStatus
		body = `{"users":[{"id":"admin-test-user","email":"admin@example.test"}],"limit":50,"offset":0}`
	}
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

func (d *adminAdapterDoer) snapshot() []adminAdapterCall {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]adminAdapterCall(nil), d.calls...)
}

func adminProbeCaptureHost() uic.Node {
	admin := uistate.UseAdminConsoleAvailable()
	uistate.CaptureAdminConsole(admin)
	return Div()
}

func waitAdminAdapterCalls(t *testing.T, d *adminAdapterDoer, want int) []adminAdapterCall {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		calls := d.snapshot()
		if len(calls) >= want {
			return calls
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("admin HTTP calls did not reach %d; got %+v", want, d.snapshot())
	return nil
}

func setupAdminAdapterIdentity(t *testing.T, configuredToken, rotatedToken string) {
	t.Helper()
	oldApp := appstate.Default
	appstate.Default = nil
	oldPrefs := uistate.LoadPrefs()
	p := prefs.Default()
	p.ServerURL = "https://admin-adapter.example.test"
	p.ServerToken = configuredToken
	p.ServerMode = prefs.ServerSelfHosted
	if err := uistate.PersistPrefs(p); err != nil {
		appstate.Default = oldApp
		t.Fatalf("persist isolated admin prefs: %v", err)
	}
	key := uistate.AuthAccessTokenKey
	oldToken := browserstore.GetString(key)
	browserstore.SetMemory(key, rotatedToken)
	t.Cleanup(func() {
		appstate.Default = nil
		_ = uistate.PersistPrefs(oldPrefs)
		appstate.Default = oldApp
		browserstore.SetMemory(key, oldToken)
	})
}

func TestAdminProbeAndRouteShareClientWithoutChangingTokenPolicy(t *testing.T) {
	for _, tc := range []struct {
		name          string
		configured    string
		rotated       string
		status        int
		wantCalls     int
		wantAvailable bool
		wantCached    bool
		wantProbeAuth string
		wantRouteAuth string
	}{
		{
			name:       "same identity reuses overview and reports freshness",
			configured: "configured-admin-token", wantCalls: 2,
			wantAvailable: true, wantCached: true,
			wantProbeAuth: "Bearer configured-admin-token", wantRouteAuth: "Bearer configured-admin-token",
		},
		{
			name:       "rotated probe credential remains distinct from configured screen credential",
			configured: "configured-admin-token", rotated: "rotated-session-token", wantCalls: 3,
			wantAvailable: true, wantCached: false,
			wantProbeAuth: "Bearer rotated-session-token", wantRouteAuth: "Bearer configured-admin-token",
		},
		{
			name:       "probe exact 200 and screen accepts successful 2xx",
			configured: "configured-admin-token", status: http.StatusCreated, wantCalls: 2,
			wantAvailable: false, wantCached: true,
			wantProbeAuth: "Bearer configured-admin-token", wantRouteAuth: "Bearer configured-admin-token",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupAdminAdapterIdentity(t, tc.configured, tc.rotated)
			oldClient := adminclient.Default
			doer := &adminAdapterDoer{overviewStatus: tc.status, usersStatus: http.StatusOK}
			fixedNow := time.Date(2026, 10, 21, 9, 30, 0, 0, time.UTC)
			adminclient.Default = adminclient.NewClientWithClock(doer, func() time.Time { return fixedNow })
			t.Cleanup(func() { adminclient.Default = oldClient })

			f := render.New(t)
			defer f.Cleanup()
			f.Render(uic.CreateElement(adminProbeCaptureHost))
			f.Stabilize()
			uistate.SetAdminConsoleAvailable(false)

			probeAdminAccess()
			calls := waitAdminAdapterCalls(t, doer, 1)
			deadline := time.Now().Add(3 * time.Second)
			for uistate.AdminConsoleAvailable() != tc.wantAvailable && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			if got := uistate.AdminConsoleAvailable(); got != tc.wantAvailable {
				t.Fatalf("probe admin availability = %v, want %v", got, tc.wantAvailable)
			}
			if len(calls) != 1 || calls[0].Path != "/v1/admin/overview" || calls[0].Authorization != tc.wantProbeAuth {
				t.Fatalf("probe request = %+v, want overview with %q", calls, tc.wantProbeAuth)
			}

			f.Render(uic.CreateElement(screens.AdminConsole))
			f.Stabilize()
			calls = waitAdminAdapterCalls(t, doer, tc.wantCalls)
			f.Stabilize()
			if len(calls) != tc.wantCalls {
				t.Fatalf("request count = %d, want %d: %+v", len(calls), tc.wantCalls, calls)
			}
			overviewByAuth := map[string]int{}
			usersCalls := 0
			overviewCalls := 0
			for _, call := range calls {
				switch call.Path {
				case "/v1/admin/overview":
					overviewCalls++
					overviewByAuth[call.Authorization]++
				case "/v1/admin/users":
					usersCalls++
					if call.Query != "limit=50&offset=0" || call.Authorization != tc.wantRouteAuth {
						t.Fatalf("users request = %+v, want configured token and first page", call)
					}
				default:
					t.Fatalf("unexpected admin request: %+v", call)
				}
			}
			wantOverviewCalls := 1 // same-identity screen overview is served from the probe's cache.
			if tc.rotated != "" {
				wantOverviewCalls++
				if overviewByAuth[tc.wantProbeAuth] != 1 {
					t.Fatalf("probe overview calls for %q = %d, want 1: %+v", tc.wantProbeAuth, overviewByAuth[tc.wantProbeAuth], calls)
				}
			}
			if overviewCalls != wantOverviewCalls {
				t.Fatalf("overview network calls = %d, want %d for probe=%q route=%q: %+v", overviewCalls, wantOverviewCalls, tc.wantProbeAuth, tc.wantRouteAuth, calls)
			}
			if overviewByAuth[tc.wantRouteAuth] != 1 || usersCalls != 1 {
				t.Fatalf("configured-token overview network calls=%d users calls=%d, want 1 each: %+v", overviewByAuth[tc.wantRouteAuth], usersCalls, calls)
			}
			if !strings.Contains(f.Text(), "admin@example.test") {
				t.Fatalf("actual AdminConsole did not render fetched users: %s", f.Text())
			}
			cachedLabel := strings.Contains(f.Text(), "cached within the last 30 seconds")
			if cachedLabel != tc.wantCached {
				t.Fatalf("cache freshness label present=%v, want %v; text=%s", cachedLabel, tc.wantCached, f.Text())
			}
			if got := byAdminTestID(f, "admin-fetch-meta-overview"); got == "" {
				t.Fatal("actual AdminConsole omitted overview fetch timestamp")
			}
		})
	}
}

func byAdminTestID(f *render.Fixture, testID string) string {
	for _, tag := range []string{"p", "span", "div"} {
		for _, node := range f.AllByTag(tag) {
			if node.Attr("data-testid") == testID {
				return node.Text()
			}
		}
	}
	return ""
}
