// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
)

func TestAdminPendingAccessApproveCreatesRedeemableAccount(t *testing.T) {
	adminToken := "admin-secret"
	mux, store := newAdminTestMux(t, resolvedAdminID(adminToken))
	deviceID, _, err := store.MintPendingDevice("Cam's browser", time.Now().UTC())
	if err != nil {
		t.Fatalf("mint pending device: %v", err)
	}

	list := adminReq(t, mux, http.MethodGet, "/v1/admin/pending-devices", adminToken, "")
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d body %q", list.Code, list.Body.String())
	}
	var pending []AdminPendingDeviceResponse
	if err := json.NewDecoder(list.Body).Decode(&pending); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(pending) != 1 || pending[0].DeviceID != deviceID || pending[0].Label != "Cam's browser" {
		t.Fatalf("pending = %+v", pending)
	}

	approve := adminReq(t, mux, http.MethodPost, "/v1/admin/pending-devices/"+deviceID+"/approve", adminToken, "")
	if approve.Code != http.StatusOK {
		t.Fatalf("approve status = %d body %q", approve.Code, approve.Body.String())
	}
	var decision AdminPendingDeviceDecisionResponse
	if err := json.NewDecoder(approve.Body).Decode(&decision); err != nil {
		t.Fatalf("decode approval: %v", err)
	}
	if !decision.OK || decision.Action != "approve" || decision.DeviceID != deviceID || strings.TrimSpace(decision.PairingCode) == "" {
		t.Fatalf("decision = %+v", decision)
	}

	service := newAuthService(store, withSessionKey(t, Config{Token: adminToken}, store))
	session, err := service.RedeemPairingCode(context.Background(), backendrpc.RedeemPairingCodeRequest{
		PairingCode: decision.PairingCode,
		DeviceLabel: "approved-browser",
	})
	if err != nil {
		t.Fatalf("redeem approved code: %v", err)
	}
	if session.AccessToken == "" || session.RefreshToken == "" {
		t.Fatalf("session = %+v", session)
	}
	if again := adminReq(t, mux, http.MethodPost, "/v1/admin/pending-devices/"+deviceID+"/approve", adminToken, ""); again.Code != http.StatusPreconditionFailed {
		t.Fatalf("second approval status = %d, want 412; body %q", again.Code, again.Body.String())
	}
}

func TestAdminPendingAccessRejectCreatesNoAccount(t *testing.T) {
	adminToken := "admin-secret"
	mux, store := newAdminTestMux(t, resolvedAdminID(adminToken))
	deviceID, _, err := store.MintPendingDevice("Unknown laptop", time.Now().UTC())
	if err != nil {
		t.Fatalf("mint pending device: %v", err)
	}
	reject := adminReq(t, mux, http.MethodPost, "/v1/admin/pending-devices/"+deviceID+"/reject", adminToken, "")
	if reject.Code != http.StatusOK {
		t.Fatalf("reject status = %d body %q", reject.Code, reject.Body.String())
	}
	device, ok, err := store.GetPendingDevice(deviceID)
	if err != nil || !ok || device.Status != PendingDeviceStatusRejected {
		t.Fatalf("rejected device = %+v ok=%v err=%v", device, ok, err)
	}
	overview, err := store.AdminOverview(time.Now().UTC())
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if overview.TotalUsers != 0 {
		t.Fatalf("rejection created %d users, want 0", overview.TotalUsers)
	}

	unauthorized := adminReq(t, mux, http.MethodGet, "/v1/admin/pending-devices", "not-the-admin-token", "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want 401", unauthorized.Code)
	}
}
