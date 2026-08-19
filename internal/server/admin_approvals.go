// SPDX-License-Identifier: MIT

package server

import (
	"errors"
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
	// Defensive, not load-bearing: opaque is freshly random so this id has no
	// history. Kept so that every door which deliberately creates an account
	// clears the tombstone, and none of them relies on "this id cannot collide"
	// staying true.
	if err := store.ClearAccountTombstone(userID); err != nil {
		return false, "", fmt.Errorf("server approval: clear account tombstone: %w", err)
	}
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
	// C700: record WHICH account this approval produced. Without this the new
	// device:<random> user has no link back to the request that created it, so
	// the console shows an account nobody can explain — the shape of the
	// duplicate-account incident this ticket came from.
	if err := store.SetPendingDeviceAccount(deviceID, userID, ResolvedByAdmin, now); err != nil {
		// The approval itself stands; only the link failed. Undoing a granted
		// approval over a bookkeeping error would be a worse outcome than an
		// unlabelled row, and the pairing code has already been minted.
		return true, code, nil
	}
	return true, code, nil
}

// attachPendingDeviceToUser approves a request onto an account that ALREADY
// EXISTS, instead of minting a new one.
//
// This is the ticket's central fix (C700). Approval had exactly one behaviour —
// create a fresh device:<random> user — so an operator faced with "this is my
// other browser, it should be my existing account" had no way to say so. Taking
// the only available action produced a second account that could not read the
// first one's workspaces, and the server then correctly answered `workspace not
// found` forever.
//
// The target must exist and must not be suspended: pairing onto a suspended
// account would be a way to walk a suspension back without lifting it.
//
// It must also not be somebody ELSE's privileged account. Attaching mints a
// working pairing code for the target, and RedeemPairingCode is unauthenticated
// by design — so without this guard an operator holding admin-tier trust could
// request a pairing as an anonymous device, attach it to the owner account, and
// redeem full owner credentials for themselves. That is a real escalation and
// not one the flat admin model already grants, because it produces a durable
// session for another identity rather than an audited action taken as your own.
//
// The legitimate case is preserved: callerID may always attach a device to their
// OWN account, whatever its role — an owner pairing their second browser is
// exactly what this feature is for.
func attachPendingDeviceToUser(store *Store, deviceID, userID, callerID string, now time.Time) (bool, string, error) {
	if store == nil {
		return false, "", fmt.Errorf("server approval: store is not configured")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return false, "", fmt.Errorf("server approval: target account is required")
	}
	target, found, err := store.GetUserByID(userID)
	if err != nil {
		return false, "", fmt.Errorf("server approval: target lookup: %w", err)
	}
	if !found {
		return false, "", errNoSuchAccount
	}
	if target.Role == RoleOwner && strings.TrimSpace(callerID) != userID {
		return false, "", errAccountPrivileged
	}
	if suspended, err := store.IsUserSuspended(userID); err != nil {
		return false, "", fmt.Errorf("server approval: suspension check: %w", err)
	} else if suspended {
		return false, "", errAccountSuspended
	}
	code, _, err := store.MintPairingCode(userID, now)
	if err != nil {
		return false, "", fmt.Errorf("server approval: mint pairing code: %w", err)
	}
	approved, err := store.ApprovePendingDevice(deviceID, code, now)
	if err != nil {
		return false, "", fmt.Errorf("server approval: approve request: %w", err)
	}
	if !approved {
		// Nothing to compensate: unlike the new-account path this created no
		// user, and the unused code expires on its own in five minutes.
		return false, "", nil
	}
	if err := store.SetPendingDeviceAccount(deviceID, userID, ResolvedByAdmin, now); err != nil {
		return true, code, nil
	}
	return true, code, nil
}

// errNoSuchAccount and errAccountSuspended let the handler answer with a precise
// reason instead of a generic failure — an operator attaching a device to the
// wrong account id needs to know which of the two things went wrong.
var (
	errNoSuchAccount    = errors.New("server approval: no such account")
	errAccountSuspended = errors.New("server approval: account is suspended")
	// errAccountPrivileged blocks minting a session for somebody else's owner
	// account. Adversarial review, 2026-08-17: without it, admin-tier trust
	// converts into durable owner credentials in two unauthenticated calls.
	errAccountPrivileged = errors.New("server approval: that account is privileged")
)
