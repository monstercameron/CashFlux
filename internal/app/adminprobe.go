// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/monstercameron/CashFlux/internal/uistate"
)

// adminOverviewCache caches the last successful overview response so the admin
// screen can render without a second network round-trip. Nil until a successful
// probe; updated by probeAdminAccess on each reload.
var adminOverviewCache *adminOverviewResponse

// adminOverviewResponse mirrors the server's AdminOverviewResponse JSON shape
// (internal/server/admin.go AdminOverviewResponse).
type adminOverviewResponse struct {
	TotalUsers        int64  `json:"totalUsers"`
	SubsActive        int64  `json:"subsActive"`
	SubsTrialing      int64  `json:"subsTrialing"`
	SubsPastDue       int64  `json:"subsPastDue"`
	SubsCanceled      int64  `json:"subsCanceled"`
	EstimatedMRRCents int64  `json:"estimatedMrrCents"`
	TotalBlobBytes    int64  `json:"totalBlobBytes"`
	TodayRequests     int64  `json:"todayRequests"`
	TodayTokens       int64  `json:"todayTokens"`
	Day               string `json:"day"`
}

// probeAdminAccess fires a non-blocking GET /v1/admin/overview. On HTTP 200 it
// sets uistate.AdminConsoleAvailable true (and caches the overview); on any
// other outcome it leaves the atom false so the nav entry stays hidden.
func probeAdminAccess() {
	pr := uistate.LoadPrefs().Normalize()
	endpoint := strings.TrimSpace(pr.ServerURL)
	token := strings.TrimSpace(pr.ServerToken)
	if endpoint == "" || token == "" {
		return
	}
	endpoint = normalizedBackendEndpoint(endpoint)
	go func() {
		req, err := http.NewRequest(http.MethodGet, endpoint+"/v1/admin/overview", nil)
		if err != nil {
			return
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			slog.Debug("admin probe: network error", "err", err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			slog.Debug("admin probe: not admin", "status", resp.StatusCode)
			return
		}
		var ov adminOverviewResponse
		if err := json.NewDecoder(resp.Body).Decode(&ov); err != nil {
			slog.Warn("admin probe: decode error", "err", err)
			return
		}
		adminOverviewCache = &ov
		uistate.SetAdminConsoleAvailable(true)
		slog.Debug("admin probe: admin access confirmed")
	}()
}
