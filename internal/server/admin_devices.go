// SPDX-License-Identifier: MIT

package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// This file gives the operator console the two things it could not see and the
// three things it could not do (TODOS.md C698, C700).
//
// Could not see: which account a browser was approved onto, which workspaces
// that account owns, whether its pairing code was ever redeemed, when it was
// last active. The Users table showed account/provider/plan/status/date, none of
// which identifies a device — so "which of these is the browser in front of me?"
// had no answer, and neither did "why can this browser not read my data?".
//
// Could not do: attach a device to an account that already exists, reissue a
// lapsed pairing code, or remove a provisional account that should never have
// been created. Only one action existed — approve, which always minted a NEW
// account — so the sole available response to a mis-paired browser was to create
// another mis-paired browser.

// AdminDeviceResponse is one device request's full lifecycle, for the console.
// The pairing code itself is never included: it is shown once, at approval, and
// serving it back from a list endpoint would turn an audit view into a place to
// harvest credentials.
type AdminDeviceResponse struct {
	DeviceID    string `json:"deviceId"`
	Label       string `json:"label"`
	Status      string `json:"status"`
	UserID      string `json:"userId,omitempty"`
	RequestedAt string `json:"requestedAt"`
	ExpiresAt   string `json:"expiresAt"`
	ResolvedAt  string `json:"resolvedAt,omitempty"`
	ResolvedBy  string `json:"resolvedBy,omitempty"`
	RedeemedAt  string `json:"redeemedAt,omitempty"`
	// HasPairingCode says a code is currently attached, without disclosing it.
	HasPairingCode bool `json:"hasPairingCode"`
	// Reissuable says the console may offer "Reissue pairing" for this row, so
	// the button's availability comes from the server's own rules rather than
	// the console re-deriving them and drifting.
	Reissuable bool `json:"reissuable"`
}

func deviceResponse(d DeviceLifecycle, now time.Time) AdminDeviceResponse {
	status := d.EffectiveStatus(now)
	return AdminDeviceResponse{
		DeviceID:       d.DeviceID,
		Label:          d.Label,
		Status:         status,
		UserID:         d.UserID,
		RequestedAt:    formatTime(d.RequestedAt),
		ExpiresAt:      formatTime(d.ExpiresAt),
		ResolvedAt:     formatOptionalTime(d.ResolvedAt),
		ResolvedBy:     d.ResolvedBy,
		RedeemedAt:     formatOptionalTime(d.RedeemedAt),
		HasPairingCode: d.PairingCodeIssued,
		Reissuable: strings.TrimSpace(d.UserID) != "" &&
			(status == PendingDeviceStatusApproved || status == PendingDeviceStatusExpired),
	}
}

// AdminWorkspaceResponse is one workspace as the console sees it — the mapping
// that was entirely absent before, and without which a `workspace not found`
// report cannot be checked against anything.
type AdminWorkspaceResponse struct {
	WorkspaceID string `json:"workspaceId"`
	Name        string `json:"name"`
	Deleted     bool   `json:"deleted"`
	Version     int64  `json:"version"`
	UpdatedAt   string `json:"updatedAt"`
	// DeviceID is the device that last wrote this workspace, which is how a
	// browser in front of you is matched to a row in this list.
	DeviceID string `json:"deviceId,omitempty"`
}

// AdminAccessView is the read-only "what this device can access" answer C698
// requires BEFORE any transfer action is offered. Showing it first is the whole
// point: a migration decided from a guess about which workspace belongs to whom
// is how data gets moved onto the wrong account.
type AdminAccessView struct {
	UserID     string                   `json:"userId"`
	Username   string                   `json:"username,omitempty"`
	Email      string                   `json:"email,omitempty"`
	Provider   string                   `json:"provider"`
	Role       string                   `json:"role"`
	Suspended  bool                     `json:"suspended"`
	CreatedAt  string                   `json:"createdAt"`
	Workspaces []AdminWorkspaceResponse `json:"workspaces"`
	Devices    []AdminDeviceResponse    `json:"devices"`
	// LastSyncAt is the most recent write across the account's workspaces — the
	// closest honest answer to "is this account actually in use?". Empty when
	// nothing has ever been written.
	LastSyncAt string `json:"lastSyncAt,omitempty"`
}

// handleAdminDevices serves GET /v1/admin/devices — every device request with
// its lifecycle, newest first, optionally filtered to one account.
func handleAdminDevices(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := adminAuthorize(cfg, store, w, r, "admin.device.list", "devices"); !ok {
			return
		}
		now := time.Now().UTC()
		var (
			devices []DeviceLifecycle
			err     error
		)
		if userID := strings.TrimSpace(r.URL.Query().Get("userId")); userID != "" {
			devices, err = store.DeviceLifecyclesForUser(userID)
		} else {
			limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
			devices, err = store.ListDeviceLifecycles(limit)
		}
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "device lookup failed")
			return
		}
		out := make([]AdminDeviceResponse, 0, len(devices))
		for _, d := range devices {
			out = append(out, deviceResponse(d, now))
		}
		writeJSON(w, out)
	}
}

// handleAdminUserAccess serves GET /v1/admin/users/{id}/access — the read-only
// view of everything one account can reach.
func handleAdminUserAccess(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := adminAuthorize(cfg, store, w, r, "admin.user.access", "user"); !ok {
			return
		}
		userID := strings.TrimSpace(r.PathValue("id"))
		if userID == "" {
			writeErrorJSON(w, ErrorReasonInvalidArgument, "user id is required")
			return
		}
		view, found, err := buildAccessView(store, userID, time.Now().UTC())
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "access lookup failed")
			return
		}
		if !found {
			writeErrorJSON(w, ErrorReasonNotFound, "no such account")
			return
		}
		writeJSON(w, view)
	}
}

// buildAccessView assembles one account's reachable surface.
func buildAccessView(store *Store, userID string, now time.Time) (AdminAccessView, bool, error) {
	user, found, err := store.GetUserByID(userID)
	if err != nil || !found {
		return AdminAccessView{}, found, err
	}
	suspended, err := store.IsUserSuspended(userID)
	if err != nil {
		return AdminAccessView{}, true, err
	}
	// Deleted workspaces are INCLUDED deliberately. A tombstoned workspace still
	// owns its id, and a device pinned to that id still gets `workspace not
	// found` — hiding the row would hide the explanation.
	workspaces, err := store.ListWorkspaces(userID, true)
	if err != nil {
		return AdminAccessView{}, true, err
	}
	devices, err := store.DeviceLifecyclesForUser(userID)
	if err != nil {
		return AdminAccessView{}, true, err
	}
	view := AdminAccessView{
		UserID:    user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Provider:  user.Provider,
		Role:      user.Role,
		Suspended: suspended,
		CreatedAt: formatTime(user.CreatedAt),
	}
	var lastSync time.Time
	for _, ws := range workspaces {
		view.Workspaces = append(view.Workspaces, AdminWorkspaceResponse{
			WorkspaceID: ws.ID,
			Name:        ws.Name,
			Deleted:     ws.Deleted,
			Version:     ws.Version,
			UpdatedAt:   formatTime(ws.UpdatedAt),
			DeviceID:    ws.DeviceID,
		})
		if ws.UpdatedAt.After(lastSync) {
			lastSync = ws.UpdatedAt
		}
	}
	for _, d := range devices {
		view.Devices = append(view.Devices, deviceResponse(d, now))
	}
	if !lastSync.IsZero() {
		view.LastSyncAt = formatTime(lastSync)
	}
	if view.Workspaces == nil {
		view.Workspaces = []AdminWorkspaceResponse{}
	}
	if view.Devices == nil {
		view.Devices = []AdminDeviceResponse{}
	}
	return view, true, nil
}

// adminAttachDeviceRequest names the account a pending device should join.
type adminAttachDeviceRequest struct {
	UserID string `json:"userId"`
}

// handleAdminPendingDeviceAttach serves
// POST /v1/admin/pending-devices/{id}/attach — approve a request onto an
// EXISTING account rather than minting a new one.
func handleAdminPendingDeviceAttach(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin, ok := adminAuthorize(cfg, store, w, r, "admin.access.attach", "pending_device")
		if !ok {
			return
		}
		deviceID := strings.TrimSpace(r.PathValue("id"))
		if deviceID == "" {
			writeErrorJSON(w, ErrorReasonInvalidArgument, "device id is required")
			return
		}
		var req adminAttachDeviceRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
			writeErrorJSON(w, ErrorReasonInvalidArgument, "invalid request body")
			return
		}
		attached, code, err := attachPendingDeviceToUser(store, deviceID, req.UserID, time.Now().UTC())
		switch {
		case errors.Is(err, errNoSuchAccount):
			writeErrorJSON(w, ErrorReasonNotFound, "no such account")
			return
		case errors.Is(err, errAccountSuspended):
			// Refused rather than silently allowed: pairing a new device onto a
			// suspended account would be a way around the suspension.
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "account is suspended")
			return
		case err != nil:
			writeErrorJSON(w, ErrorReasonInternal, "attach failed")
			return
		}
		if !attached {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "access request is no longer pending")
			return
		}
		auditFromRequest(r, store, admin, "admin.access.attach", "pending_device", deviceID+"->"+strings.TrimSpace(req.UserID))
		writeJSON(w, AdminPendingDeviceDecisionResponse{
			OK: true, Action: "attach", DeviceID: deviceID, PairingCode: code,
		})
	}
}

// handleAdminPendingDeviceReissue serves
// POST /v1/admin/pending-devices/{id}/reissue — mint a fresh pairing code for an
// approval whose code lapsed before anyone typed it.
//
// Without this the only recovery is to request again, which mints another
// account. That is how one browser ends up owning two.
func handleAdminPendingDeviceReissue(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin, ok := adminAuthorize(cfg, store, w, r, "admin.access.reissue", "pending_device")
		if !ok {
			return
		}
		deviceID := strings.TrimSpace(r.PathValue("id"))
		if deviceID == "" {
			writeErrorJSON(w, ErrorReasonInvalidArgument, "device id is required")
			return
		}
		code, ok, err := store.ReissuePairingCode(deviceID, time.Now().UTC())
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "reissue failed")
			return
		}
		if !ok {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "this request has no account to pair, or its code was already used")
			return
		}
		auditFromRequest(r, store, admin, "admin.access.reissue", "pending_device", deviceID)
		writeJSON(w, AdminPendingDeviceDecisionResponse{
			OK: true, Action: "reissue", DeviceID: deviceID, PairingCode: code,
		})
	}
}

// AdminProvisionalDeleteResponse reports the removal of a provisional account.
type AdminProvisionalDeleteResponse struct {
	OK     bool   `json:"ok"`
	UserID string `json:"userId"`
}

// handleAdminUserDeleteProvisional serves
// DELETE /v1/admin/users/{id}/provisional — remove a device account that was
// created by an approval and never became anything.
//
// It is a separate door from the ordinary account delete, and deliberately
// narrower: it refuses any account that is not a device: account, and any
// account that owns a workspace with data. Those two guards are the difference
// between "clean up a mistake" and "delete somebody's household by clicking the
// wrong row" — and this endpoint exists precisely to be used in a moment of
// confusion about which account is which.
func handleAdminUserDeleteProvisional(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin, ok := adminAuthorize(cfg, store, w, r, "admin.user.delete_provisional", "user")
		if !ok {
			return
		}
		userID := strings.TrimSpace(r.PathValue("id"))
		if userID == "" {
			writeErrorJSON(w, ErrorReasonInvalidArgument, "user id is required")
			return
		}
		if userID == admin.ID {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "you cannot delete your own account")
			return
		}
		user, found, err := store.GetUserByID(userID)
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "account lookup failed")
			return
		}
		if !found {
			writeErrorJSON(w, ErrorReasonNotFound, "no such account")
			return
		}
		if !strings.HasPrefix(user.ID, "device:") {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, "only provisional device accounts can be removed this way")
			return
		}
		workspaces, err := store.ListWorkspaces(userID, true)
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "workspace lookup failed")
			return
		}
		for _, ws := range workspaces {
			if ws.Deleted {
				continue
			}
			// A workspace that has been written to holds somebody's records.
			// Refuse and say so, rather than deciding on their behalf that a
			// device account's data does not count.
			writeErrorJSON(w, ErrorReasonFailedPrecondition,
				"this account owns live workspace data — migrate or transfer it before removing the account")
			return
		}
		if _, err := store.DeleteAccount(userID); err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "account removal failed")
			return
		}
		auditFromRequest(r, store, admin, "admin.user.delete_provisional", "user", userID)
		writeJSON(w, AdminProvisionalDeleteResponse{OK: true, UserID: userID})
	}
}
