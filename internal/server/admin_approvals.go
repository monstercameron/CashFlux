// SPDX-License-Identifier: MIT

package server

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// AdminPendingDeviceResponse is one unresolved client access request. Pairing
// codes are deliberately absent until the operator approves the request.
type AdminPendingDeviceResponse struct {
	DeviceID    string `json:"deviceId"`
	Label       string `json:"label"`
	RequestedAt string `json:"requestedAt"`
	ExpiresAt   string `json:"expiresAt"`
}

// AdminPendingDeviceDecisionResponse reports an operator decision. PairingCode
// is returned exactly once on approval so the console can cross-check it with
// the waiting client.
type AdminPendingDeviceDecisionResponse struct {
	OK          bool   `json:"ok"`
	Action      string `json:"action"`
	DeviceID    string `json:"deviceId"`
	PairingCode string `json:"pairingCode,omitempty"`
}

func handleAdminPendingDevices(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := adminAuthorize(cfg, store, w, r, "admin.access.list", "pending_devices"); !ok {
			return
		}
		devices, err := store.ListPendingDevices(time.Now().UTC())
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "pending access lookup failed")
			return
		}
		out := make([]AdminPendingDeviceResponse, 0, len(devices))
		for _, device := range devices {
			out = append(out, AdminPendingDeviceResponse{
				DeviceID:    device.DeviceID,
				Label:       device.Label,
				RequestedAt: formatTime(device.RequestedAt),
				ExpiresAt:   formatTime(device.ExpiresAt),
			})
		}
		writeJSON(w, out)
	}
}

func handleAdminPendingDeviceApprove(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin, ok := adminAuthorize(cfg, store, w, r, "admin.access.approve", "pending_device")
		if !ok {
			return
		}
		deviceID := strings.TrimSpace(r.PathValue("id"))
		if deviceID == "" {
			writeErrorJSON(w, ErrorReasonInvalidArgument, "device id is required")
			return
		}
		approved, code, err := approvePendingDeviceAsNewUser(store, deviceID, time.Now().UTC())
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "access approval failed")
			return
		}
		if !approved {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "access request is no longer pending")
			return
		}
		auditFromRequest(r, store, admin, "admin.access.approve", "pending_device", deviceID)
		writeJSON(w, AdminPendingDeviceDecisionResponse{
			OK: true, Action: "approve", DeviceID: deviceID, PairingCode: code,
		})
	}
}

func handleAdminPendingDeviceReject(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin, ok := adminAuthorize(cfg, store, w, r, "admin.access.reject", "pending_device")
		if !ok {
			return
		}
		deviceID := strings.TrimSpace(r.PathValue("id"))
		if deviceID == "" {
			writeErrorJSON(w, ErrorReasonInvalidArgument, "device id is required")
			return
		}
		rejected, err := store.RejectPendingDevice(deviceID)
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "access rejection failed")
			return
		}
		if !rejected {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "access request is no longer pending")
			return
		}
		auditFromRequest(r, store, admin, "admin.access.reject", "pending_device", deviceID)
		writeJSON(w, AdminPendingDeviceDecisionResponse{
			OK: true, Action: "reject", DeviceID: deviceID,
		})
	}
}

// approvePendingDeviceAsNewUser creates the distinct account an approved
// request will redeem. If another operator resolved the request first, every
// provisional row/code created here is removed before returning.
func approvePendingDeviceAsNewUser(store *Store, deviceID string, now time.Time) (bool, string, error) {
	if store == nil {
		return false, "", fmt.Errorf("server approval: store is not configured")
	}
	opaque, err := randomURLToken(18)
	if err != nil {
		return false, "", fmt.Errorf("server approval: generate account id: %w", err)
	}
	userID := "device:" + opaque
	if err := store.UpsertUser(User{
		ID: userID, Provider: "device", Subject: opaque, CreatedAt: now,
	}); err != nil {
		return false, "", fmt.Errorf("server approval: create account: %w", err)
	}
	cleanup := func() {
		_, _ = store.DeleteAccount(userID)
	}
	code, _, err := store.MintPairingCode(userID, now)
	if err != nil {
		cleanup()
		return false, "", fmt.Errorf("server approval: mint pairing code: %w", err)
	}
	approved, err := store.ApprovePendingDevice(deviceID, code, now)
	if err != nil {
		cleanup()
		return false, "", fmt.Errorf("server approval: approve request: %w", err)
	}
	if !approved {
		cleanup()
		return false, "", nil
	}
	return true, code, nil
}
