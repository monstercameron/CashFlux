// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"log/slog"
	"strings"

	"github.com/monstercameron/CashFlux/internal/adminclient"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// probeAdminAccess fires a non-blocking GET /v1/admin/overview. On HTTP 200 it
// sets uistate.AdminConsoleAvailable true; on any other outcome it leaves the
// atom false so the nav entry stays hidden. The shared client supplies successful
// overview data to /admin for the same endpoint/token identity.
func probeAdminAccess() {
	pr := uistate.LoadPrefs().Normalize()
	endpoint := strings.TrimSpace(pr.ServerURL)
	// Preserve the probe's existing rotating-token selection. AdminConsole still
	// uses the configured prefs token; the shared cache is keyed by token identity,
	// so those two paths cannot accidentally share a response across credentials.
	token := strings.TrimSpace(effectiveServerToken(pr))
	if endpoint == "" || token == "" || validateBackendTransport(endpoint) != nil {
		return
	}
	endpoint = normalizedBackendEndpoint(endpoint)
	go func() {
		result, err := adminclient.Default.Overview(context.Background(), endpoint, token)
		if err != nil {
			slog.Debug("admin overview fetch error", "err", err)
			return
		}
		// The probe historically requires exactly HTTP 200. Keep that gate even
		// though the screen accepts any successful 2xx response.
		if result.Status != 200 {
			slog.Debug("admin probe: not admin", "status", result.Status)
			return
		}
		uistate.SetAdminConsoleAvailable(true)
		slog.Debug("admin probe: admin access confirmed")
	}()
}
